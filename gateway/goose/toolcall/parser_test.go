package toolcall

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func raw(name, args string) json.RawMessage {
	return json.RawMessage(`{"status":"success","value":{"name":"` + name + `","arguments":` + args + `}}`)
}

// A feed of identical `shell` lines says nothing, so the detail must identify the call.
func TestParse_Detail(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"shell command", `{"command":"rg --files src"}`, "rg --files src"},
		{"file path", `{"path":"src/tools/json/logic.ts"}`, "src/tools/json/logic.ts"},
		{"snake case path", `{"file_path":"src/a.ts"}`, "src/a.ts"},
		{"search pattern", `{"pattern":"hexToRgb"}`, "hexToRgb"},
		{"unknown shape falls back to compact json", `{"weird":1}`, `{"weird":1}`},
		{"no arguments", `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := Parse(raw("developer__shell", tc.args))

			if call.Name != "developer__shell" {
				t.Errorf("Name = %q", call.Name)
			}
			if call.Detail != tc.want {
				t.Errorf("Detail = %q, want %q", call.Detail, tc.want)
			}
		})
	}
}

// A long argument must not turn one feed line into a paragraph.
func TestParse_TruncatesDetail(t *testing.T) {
	call := Parse(raw("developer__shell", `{"command":"`+strings.Repeat("x", 400)+`"}`))

	if runes := utf8.RuneCountInString(call.Detail); runes > detailChars+1 {
		t.Errorf("detail length = %d runes, want <= %d", runes, detailChars+1)
	}
	if !strings.HasSuffix(call.Detail, "…") {
		t.Errorf("a truncated detail must say so, got %q", call.Detail)
	}
}

// Malformed input must not panic or fail the run.
func TestParse_ToleratesBadShapes(t *testing.T) {
	cases := map[string]json.RawMessage{
		"empty":            nil,
		"not json":         json.RawMessage(`not json`),
		"missing value":    json.RawMessage(`{"status":"success"}`),
		"arguments as int": json.RawMessage(`{"value":{"name":"read","arguments":7}}`),
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			if call := Parse(block); call.Detail != "" {
				t.Errorf("Detail = %q, want empty", call.Detail)
			}
		})
	}
}

func TestIsToolCall(t *testing.T) {
	for _, blockType := range []string{"toolRequest", "tool_request", "tool_use"} {
		if !IsToolCall(blockType) {
			t.Errorf("%q must count as a tool call", blockType)
		}
	}
	for _, blockType := range []string{"text", "thinking", ""} {
		if IsToolCall(blockType) {
			t.Errorf("%q must not count as a tool call", blockType)
		}
	}
}
