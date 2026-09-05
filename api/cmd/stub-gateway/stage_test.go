package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStagedCannedBody_UnscriptedStageKeepsDefault(t *testing.T) {
	body := stagedCannedBody(&configuredAgent{}, "design")

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

	body := stagedCannedBody(target, "brainstorming")

	assert.Equal(t, "question_asked", body["outcome"])
	assert.Equal(t, "Which database?", body["message"])
	assert.Equal(t, "brainstorming_question", body["messageKind"])
}

func TestStagedCannedBody_ScriptLeavesUnsetFieldsAtTheDefault(t *testing.T) {
	target := &configuredAgent{script: map[string]StageScript{
		"implementation": {Outcome: "errored", ErrorReason: "stub_failure"},
	}}

	body := stagedCannedBody(target, "implementation")

	assert.Equal(t, "errored", body["outcome"])
	assert.Equal(t, "stub_failure", body["errorReason"])
	assert.Equal(t, "https://github.com/example/repo/pull/1", body["prUrl"],
		"an unset script field must fall through to the canned default")
}
