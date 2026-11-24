package common

import (
	"strings"
	"testing"
)

func renderThreadPrompt(t *testing.T, thread string) string {
	t.Helper()
	task, err := parseStageExecutePayload(unmarshalEvent(t, threadPayload(thread)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	return prompt
}

func TestRenderPrompt_AnchoredCommentsShowWhatTheyAreAbout(t *testing.T) {
	prompt := renderThreadPrompt(t, `[
		{
			"idMessage": 12,
			"message": "## Plan\n1. Add column\n2. Backfill",
			"messageKind": "comment",
			"createdAt": "2026-11-23T10:00:00Z",
			"creator": {"name": "Bot"}
		},
		{
			"idMessage": 13,
			"message": "Why a backfill?",
			"messageKind": "comment",
			"createdAt": "2026-11-23T10:05:00Z",
			"creator": {"name": "Alice"},
			"anchor": {"idParentMessage": 12, "anchorLineStart": 3, "anchorLineEnd": 3, "isOutdated": false, "anchorText": "2. Backfill", "quote": "Backfill"}
		},
		{
			"idMessage": 14,
			"message": "Split this step.",
			"messageKind": "comment",
			"createdAt": "2026-11-23T10:06:00Z",
			"creator": {"name": "Alice"},
			"anchor": {"idParentMessage": 12, "anchorLineStart": 1, "anchorLineEnd": 2, "isOutdated": false, "anchorText": "## Plan\n1. Add column"}
		},
		{
			"idMessage": 15,
			"message": "Old remark.",
			"messageKind": "comment",
			"createdAt": "2026-11-23T10:07:00Z",
			"creator": {"name": "Alice"},
			"anchor": {"idParentMessage": 12, "anchorLineStart": 2, "anchorLineEnd": 2, "isOutdated": true, "anchorText": "1. Add a column", "quote": "a column"}
		}
	]`)

	cases := []struct {
		name string
		want string
	}{
		{"plain comment keeps its id", `[Bot · comment · 2026-11-23T10:00:00Z · #12]: ## Plan`},
		{"quote is preferred", `[Alice · comment · 2026-11-23T10:05:00Z · #13 · on #12 "Backfill"]: Why a backfill?`},
		{"anchor text without a quote, on one line", `[Alice · comment · 2026-11-23T10:06:00Z · #14 · on #12 "## Plan 1. Add column"]: Split this step.`},
		{"outdated anchor is flagged", `#15 · on #12 "a column" (that text has changed since)]: Old remark.`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if !strings.Contains(prompt, testCase.want) {
				t.Errorf("prompt does not contain %q:\n%s", testCase.want, prompt)
			}
		})
	}
}

func TestRenderPrompt_MessageWithoutIdKeepsThePlainHeader(t *testing.T) {
	prompt := renderThreadPrompt(t, `[
		{"message": "Looks good.", "messageKind": "comment", "createdAt": "2026-05-26T10:00:00Z", "creator": {"name": "Alice"}}
	]`)
	want := "[Alice · comment · 2026-05-26T10:00:00Z]: Looks good."
	if !strings.Contains(prompt, want) {
		t.Errorf("prompt does not contain %q:\n%s", want, prompt)
	}
}
