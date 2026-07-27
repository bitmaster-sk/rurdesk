package controller

import (
	"regexp"
	"strconv"
	"strings"
)

// mentionTokenRe matches @[name](user:id). Shared token format with the frontend.
var mentionTokenRe = regexp.MustCompile(`@\[[^\]]+\]\(user:(\d+)\)`)

// mentionDisplayRe matches @[Name](user:id) and captures the display name.
var mentionDisplayRe = regexp.MustCompile(`@\[([^\]]+)\]\(user:\d+\)`)

// splitCodeSpans splits text into alternating code / non-code spans, mirroring
// the frontend's splitCodeSpans in message-body.component.ts (rules must stay
// identical):
//   - count consecutive backticks at i (fence length N);
//   - find the next run of exactly N backticks as closer;
//   - a closer followed by another backtick is part of a longer fence — treat
//     the opener as plain text and keep scanning;
//   - no matching closer → rest of the string is plain text.
type codeSpan struct {
	isCode bool
	text   string
}

func splitCodeSpans(text string) []codeSpan {
	spans := make([]codeSpan, 0, 4)
	i := 0

	for i < len(text) {
		if text[i] != '`' {
			next := strings.IndexByte(text[i:], '`')
			if next == -1 {
				spans = append(spans, codeSpan{isCode: false, text: text[i:]})
				break
			}
			spans = append(spans, codeSpan{isCode: false, text: text[i : i+next]})
			i = i + next
			continue
		}

		fenceLen := 0
		for i+fenceLen < len(text) && text[i+fenceLen] == '`' {
			fenceLen++
		}
		opener := text[i : i+fenceLen]

		closerStart := strings.Index(text[i+fenceLen:], opener)
		if closerStart == -1 {
			spans = append(spans, codeSpan{isCode: false, text: text[i:]})
			break
		}
		closerStart = closerStart + i + fenceLen // absolute position

		afterCloser := closerStart + fenceLen
		if afterCloser < len(text) && text[afterCloser] == '`' {
			// Part of a longer fence, not a match: emit opener as plain text, keep scanning.
			spans = append(spans, codeSpan{isCode: false, text: opener})
			i = i + fenceLen
			continue
		}

		spans = append(spans, codeSpan{isCode: true, text: text[i:afterCloser]})
		i = afterCloser
	}

	return spans
}

// parseMentionUserIds extracts mentioned user IDs from a message body, deduped
// in first-seen order. Detection is by ID only — never by display name.
// Tokens inside code spans (fenced blocks or inline code) are ignored.
func parseMentionUserIds(body string) []int64 {
	spans := splitCodeSpans(body)

	var allMatches [][]string
	for _, span := range spans {
		if span.isCode {
			continue
		}
		allMatches = append(allMatches, mentionTokenRe.FindAllStringSubmatch(span.text, -1)...)
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
