package controller

import (
	"regexp"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/markdowntext"
)

// mentionTokenRe matches @[name](user:id). Shared token format with the frontend.
var mentionTokenRe = regexp.MustCompile(`@\[[^\]]+\]\(user:(\d+)\)`)

// mentionDisplayRe matches @[Name](user:id) and captures the display name.
var mentionDisplayRe = regexp.MustCompile(`@\[([^\]]+)\]\(user:\d+\)`)

// parseMentionUserIds extracts mentioned user IDs from a message body, deduped
// in first-seen order. Detection is by ID only — never by display name.
// Tokens inside code spans (fenced blocks or inline code) are ignored.
func parseMentionUserIds(body string) []int64 {
	spans := markdowntext.SplitCodeSpans(body)

	var allMatches [][]string
	for _, span := range spans {
		if span.IsCode {
			continue
		}
		allMatches = append(allMatches, mentionTokenRe.FindAllStringSubmatch(span.Text, -1)...)
	}

	if len(allMatches) == 0 {
		return nil
	}

	seen := make(map[int64]bool, len(allMatches))
	ids := make([]int64, 0, len(allMatches))
	for _, match := range allMatches {
		idUser, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			// Overflow or other parse error — skip this id.
			continue
		}
		if !seen[idUser] {
			seen[idUser] = true
			ids = append(ids, idUser)
		}
	}
	return ids
}

// stripMentionTokens replaces each @[Name](user:id) token with @Name, for
// human-readable notification bodies.
func stripMentionTokens(s string) string {
	return mentionDisplayRe.ReplaceAllString(s, "@$1")
}
