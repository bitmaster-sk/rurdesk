package markdowntext

import (
	"regexp"
	"strings"
)

var codePattern = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")

type CodeSpan struct {
	IsCode bool
	Text   string
}

// Keep codePattern identical to MarkdownCode.pattern in the client, or links inside code render differently than they are parsed.
func OutsideCode(text string) []string {
	return codePattern.Split(text, -1)
}

func ReplaceOutsideCode(text string, replace func(string) string) string {
	var result strings.Builder
	last := 0
	for _, match := range codePattern.FindAllStringIndex(text, -1) {
		result.WriteString(replace(text[last:match[0]]))
		result.WriteString(text[match[0]:match[1]])
		last = match[1]
	}
	result.WriteString(replace(text[last:]))
	return result.String()
}

// SplitCodeSpans splits text into alternating code / non-code spans:
//   - count consecutive backticks at i (fence length N);
//   - find the next run of exactly N backticks as closer;
//   - a closer followed by another backtick is part of a longer fence — treat
//     the opener as plain text and keep scanning;
//   - no matching closer → rest of the string is plain text.
func SplitCodeSpans(text string) []CodeSpan {
	spans := make([]CodeSpan, 0, 4)
	i := 0

	for i < len(text) {
		if text[i] != '`' {
			next := strings.IndexByte(text[i:], '`')
			if next == -1 {
				spans = append(spans, CodeSpan{IsCode: false, Text: text[i:]})
				break
			}
			spans = append(spans, CodeSpan{IsCode: false, Text: text[i : i+next]})
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
			spans = append(spans, CodeSpan{IsCode: false, Text: text[i:]})
			break
		}
		closerStart = closerStart + i + fenceLen // absolute position

		afterCloser := closerStart + fenceLen
		if afterCloser < len(text) && text[afterCloser] == '`' {
			// Part of a longer fence, not a match: emit opener as plain text, keep scanning.
			spans = append(spans, CodeSpan{IsCode: false, Text: opener})
			i = i + fenceLen
			continue
		}

		spans = append(spans, CodeSpan{IsCode: true, Text: text[i:afterCloser]})
		i = afterCloser
	}

	return spans
}
