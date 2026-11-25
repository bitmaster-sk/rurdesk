package common

import (
	"strings"
	"testing"
)

func revisionPayload(idMessageSeen string) string {
	seen := ""
	if idMessageSeen != "" {
		seen = `"idMessageSeen": ` + idMessageSeen + `,`
	}
	return `{
		"idRun": 1, "idIssue": 2, "idUserAgent": 3, "idProject": 4,
		"payload": {
			"idTask": 5, "stage": "implementation_plan", "attemptNo": 2,
			"contextBundle": {
				` + seen + `
				"isRevision": true,
				"rejectedOutput": {"message": "PLAN-V1", "messageKind": "implementation_plan"},
				"reviewThread": [
					{"idMessage": 10, "message": "Plan the history table.", "messageKind": "comment", "creator": {"name": "Alice"}},
					{"idMessage": 11, "message": "Written while the agent worked.", "messageKind": "comment", "creator": {"name": "Alice"}},
					{"idMessage": 12, "message": "", "messageKind": "implementation_plan", "creator": {"name": "Bot", "isAgent": true}},
					{"idMessage": 13, "message": "Agent note after the plan.", "messageKind": "comment", "creator": {"name": "Bot", "isAgent": true}},
					{"idMessage": 14, "message": "Use option 1.", "messageKind": "comment", "creator": {"name": "Alice"}}
				]
			}
		}
	}`
}

func newMessages(task Task) []string {
	var marked []string
	for _, message := range task.Conversation {
		if message.IsNew {
			marked = append(marked, message.Message)
		}
	}
	return marked
}

func TestParseStageExecutePayload_MarksNewFeedback(t *testing.T) {
	t.Run("people's comments above the watermark are new, agent messages never", func(t *testing.T) {
		task, err := parseStageExecutePayload(unmarshalEvent(t, revisionPayload("10")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := newMessages(task)
		want := []string{"Written while the agent worked.", "Use option 1."}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("new comments = %q, want %q", got, want)
		}
		if !task.HasNewFeedback {
			t.Error("HasNewFeedback not set")
		}
	})

	t.Run("without a watermark nothing is new", func(t *testing.T) {
		task, err := parseStageExecutePayload(unmarshalEvent(t, revisionPayload("")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := newMessages(task); len(got) != 0 || task.HasNewFeedback {
			t.Errorf("comments marked new without a watermark: %q", got)
		}
	})
}

func TestRenderPrompt_MarksNewFeedback(t *testing.T) {
	t.Run("a new comment above the rejected output is marked and the revision points to the marks", func(t *testing.T) {
		task, err := parseStageExecutePayload(unmarshalEvent(t, revisionPayload("10")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		task.Branch = "b"
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if !strings.Contains(out, "[NEW] [Alice · comment") || !strings.Contains(out, "]: Written while the agent worked.") {
			t.Errorf("the comment written during the previous attempt is not marked [NEW]:\n%s", out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "Plan the history table.") && strings.HasPrefix(line, "[NEW]") {
				t.Errorf("a comment the previous attempt saw is marked [NEW]: %s", line)
			}
			if strings.Contains(line, "Agent note") && strings.HasPrefix(line, "[NEW]") {
				t.Errorf("an agent message is marked [NEW]: %s", line)
			}
		}
		if !strings.Contains(out, "the comments marked `[NEW]` are the feedback on this version") {
			t.Errorf("the revision instruction does not point to the [NEW] comments:\n%s", out)
		}
	})

	t.Run("without a watermark the prompt has no marks", func(t *testing.T) {
		task, err := parseStageExecutePayload(unmarshalEvent(t, revisionPayload("")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		task.Branch = "b"
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if strings.Contains(out, "[NEW]") {
			t.Errorf("prompt marks comments without a watermark:\n%s", out)
		}
	})
}
