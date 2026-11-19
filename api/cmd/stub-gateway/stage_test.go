package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStagedCannedBody_UnscriptedStageKeepsDefault(t *testing.T) {
	body := stagedCannedBody(&configuredAgent{}, "design", nil)

	assert.Equal(t, "output_submitted", body["outcome"])
	assert.Equal(t, "Stub design proposal.", body["message"])
}

func TestStagedCannedBody_ScriptOverridesOutcomeAndMessage(t *testing.T) {
	target := &configuredAgent{script: map[string]StageScript{
		"brainstorming": {
			Outcome:     "question_asked",
			Message:     "Which database?",
			MessageKind: "brainstorming_question",
		},
	}}

	body := stagedCannedBody(target, "brainstorming", nil)

	assert.Equal(t, "question_asked", body["outcome"])
	assert.Equal(t, "Which database?", body["message"])
	assert.Equal(t, "brainstorming_question", body["messageKind"])
}

func TestStagedCannedBody_ScriptLeavesUnsetFieldsAtTheDefault(t *testing.T) {
	target := &configuredAgent{script: map[string]StageScript{
		"implementation": {Outcome: "errored", ErrorReason: "stub_failure"},
	}}

	body := stagedCannedBody(target, "implementation", nil)

	assert.Equal(t, "errored", body["outcome"])
	assert.Equal(t, "stub_failure", body["errorReason"])
	assert.Equal(t, "https://github.com/example/repo/pull/1", body["prUrl"],
		"an unset script field must fall through to the canned default")
}

func TestStagedCannedBody_ReleaseNotesSubmitsTheNotesWithoutAMessageKind(t *testing.T) {
	body := stagedCannedBody(&configuredAgent{}, "release_notes", nil)

	assert.Equal(t, "output_submitted", body["outcome"])
	assert.NotEmpty(t, body["message"])
	assert.NotContains(t, body, "messageKind")
}

func chatPayload(mode string, messages ...map[string]any) map[string]any {
	return chatPayloadFrom(mode, nil, messages...)
}

func chatPayloadFrom(mode string, draftFrom map[string]any, messages ...map[string]any) map[string]any {
	thread := make([]any, 0, len(messages))
	for _, message := range messages {
		thread = append(thread, message)
	}
	return map[string]any{
		"stage": "chat",
		"contextBundle": map[string]any{
			"reviewThread": thread,
			"chat":         map[string]any{"mode": mode, "draftFromMessage": draftFrom},
		},
	}
}

func TestCannedBody_Chat(t *testing.T) {
	user := func(body string) map[string]any {
		return map[string]any{"author": "user", "kind": "text", "body": body}
	}
	agentReply := map[string]any{"author": "agent", "kind": "text", "body": "Earlier answer"}

	t.Run("reply echoes the last user message", func(t *testing.T) {
		body := defaultCannedBody("chat", chatPayload("reply", user("first"), agentReply, user("hello there")))

		assert.Equal(t, "output_submitted", body["outcome"])
		assert.Equal(t, "chat_reply", body["messageKind"])
		assert.Equal(t, "Stub reply: hello there", body["message"])
	})

	t.Run("task_draft answers with a markdown draft", func(t *testing.T) {
		body := defaultCannedBody("chat", chatPayload("task_draft", user("write a task")))

		assert.Equal(t, "output_submitted", body["outcome"])
		assert.Equal(t, "task_draft", body["messageKind"])
		assert.True(t, strings.HasPrefix(body["message"].(string), "# "), "draft starts with a title heading")
	})

	t.Run("task_draft echoes the title of the quoted answer", func(t *testing.T) {
		quoted := map[string]any{"idMessage": float64(7), "body": "## Add a retry button\n\nDetails follow."}
		body := defaultCannedBody("chat", chatPayloadFrom("task_draft", quoted, user("write a task")))

		assert.Equal(t, "task_draft", body["messageKind"])
		assert.True(t, strings.HasPrefix(body["message"].(string), "# "), "draft starts with a title heading")
		assert.Contains(t, strings.SplitN(body["message"].(string), "\n", 2)[0], "Add a retry button")
	})

	t.Run("a thread without a user message still replies", func(t *testing.T) {
		body := defaultCannedBody("chat", chatPayload("reply"))

		assert.Equal(t, "chat_reply", body["messageKind"])
		assert.NotEmpty(t, body["message"])
	})
}

func TestCompletionTarget_ChatTurnCompletesWithItsRunToken(t *testing.T) {
	agentTarget := &configuredAgent{apiKey: "agent-key"}

	chat := completionTarget(agentTarget, map[string]any{"runToken": "rdr_turn"})
	workflow := completionTarget(agentTarget, map[string]any{"stage": "design"})

	assert.Equal(t, "rdr_turn", chat.apiKey)
	assert.Equal(t, "agent-key", agentTarget.apiKey, "the registered agent keeps its own key")
	assert.Equal(t, "agent-key", workflow.apiKey)
}
