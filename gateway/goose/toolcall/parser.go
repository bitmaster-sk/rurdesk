// Package toolcall parses tool invocations out of goose stream-json content blocks.
package toolcall

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

const detailChars = 400

// Tried in order; anything else falls back to the compact argument JSON.
var detailArgKeys = []string{"command", "path", "file_path", "filePath", "pattern", "query", "uri", "url"}

type Call struct {
	Name   string
	Detail string
}

// Aliases are tolerated so a goose rename does not zero the tool count.
func IsToolCall(blockType string) bool {
	switch blockType {
	case "toolRequest", "tool_request", "tool_use":
		return true
	}
	return false
}

// A bad shape yields a zero Call, never an error: the stream is telemetry.
func Parse(raw json.RawMessage) Call {
	if len(raw) == 0 {
		return Call{}
	}
	var block struct {
		Value struct {
			Name      string                     `json:"name"`
			Arguments map[string]json.RawMessage `json:"arguments"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return Call{}
	}
	return Call{Name: block.Value.Name, Detail: detail(block.Value.Arguments)}
}

func detail(arguments map[string]json.RawMessage) string {
	if len(arguments) == 0 {
		return ""
	}
	for _, key := range detailArgKeys {
		if value, ok := stringArg(arguments, key); ok {
			return truncateRunes(value, detailChars)
		}
	}
	// Keys are sorted so a tool always shows the same argument between calls.
	for _, key := range slices.Sorted(maps.Keys(arguments)) {
		if value, ok := stringArg(arguments, key); ok {
			return truncateRunes(firstLine(value), detailChars)
		}
	}
	compact, err := json.Marshal(arguments)
	if err != nil {
		return ""
	}
	return truncateRunes(string(compact), detailChars)
}

func stringArg(arguments map[string]json.RawMessage, key string) (string, bool) {
	argument, ok := arguments[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(argument, &value); err != nil || value == "" {
		return "", false
	}
	return value, true
}

func firstLine(value string) string {
	index := strings.IndexByte(value, '\n')
	if index < 0 {
		return value
	}
	return strings.TrimRight(value[:index], "\r ") + "…"
}

func truncateRunes(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "…"
}
