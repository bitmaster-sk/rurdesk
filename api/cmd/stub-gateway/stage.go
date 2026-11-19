package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const thinkingBatchDelay = 700 * time.Millisecond

// StageScript overrides what the stub reports for one stage; an absent stage keeps the canned default.
type StageScript struct {
	Outcome     string `json:"outcome"`
	Message     string `json:"message"`
	MessageKind string `json:"messageKind"`
	PrUrl       string `json:"prUrl"`
	BranchName  string `json:"branchName"`
	ErrorReason string `json:"errorReason"`
	ErrorDetail string `json:"errorDetail"`
	// Stall reports nothing for the stage, leaving the run in_progress.
	Stall         bool              `json:"stall"`
	WikiProposals []json.RawMessage `json:"wikiProposals"`
}

func executeStage(target *configuredAgent, idRun, idProject int64, payload map[string]any) {
	idTaskF, _ := payload["idTask"].(float64)
	idTask := int64(idTaskF)
	stage, _ := payload["stage"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sendHeartbeats(ctx, target, idTask)

	sendThinking(ctx, target, idTask, stage)

	if script, ok := target.script[stage]; ok && script.Stall {
		log.Printf("[stub-gw] stalling on stage=%s idRun=%d", stage, idRun)
		return
	}

	for _, proposal := range target.script[stage].WikiProposals {
		path := fmt.Sprintf("/project/%d/wiki/agent/proposal", idProject)
		if err := sendToTrackerInRun(ctx, target, idRun, http.MethodPost, path, string(proposal)); err != nil {
			log.Printf("[stub-gw] wiki proposal error: %v", err)
		}
	}

	if err := sendComplete(ctx, completionTarget(target, payload), idTask, stagedCannedBody(target, stage, payload)); err != nil {
		log.Printf("[stub-gw] complete error: %v", err)
	}
}

// sendThinking streams a stage's canned thinking the way a real gateway does: one batch at a time, each under the next seq.
func sendThinking(ctx context.Context, target *configuredAgent, idTask int64, stage string) {
	batches := []model.AgentThinkingEvents{
		{{Kind: model.ThinkingKindThinking, Text: "Picking up the " + stage + " stage. "}},
		{{Kind: model.ThinkingKindTool, Tool: "developer__shell", Text: "rg --files src"}},
		{{Kind: model.ThinkingKindThinking, Text: "That is everything I need."}},
	}
	for index, events := range batches {
		time.Sleep(thinkingBatchDelay)
		for eventIndex := range events {
			events[eventIndex].At = time.Now().UnixMilli()
		}
		body, err := json.Marshal(model.AgentThinkingReq{Seq: index + 1, Events: events})
		if err != nil {
			log.Printf("[stub-gw] thinking encode error: %v", err)
			continue
		}
		path := fmt.Sprintf("/agent/task/%d/thinking", idTask)
		if err := sendToTracker(ctx, target, http.MethodPost, path, string(body)); err != nil {
			log.Printf("[stub-gw] thinking error: %v", err)
		}
	}
}

// stagedCannedBody overlays the agent's script for a stage on that stage's canned default.
func stagedCannedBody(target *configuredAgent, stage string, payload map[string]any) map[string]any {
	body := defaultCannedBody(stage, payload)
	script, ok := target.script[stage]
	if !ok {
		return body
	}
	overrideField(body, "outcome", script.Outcome)
	overrideField(body, "message", script.Message)
	overrideField(body, "messageKind", script.MessageKind)
	overrideField(body, "prUrl", script.PrUrl)
	overrideField(body, "branchName", script.BranchName)
	overrideField(body, "errorReason", script.ErrorReason)
	overrideField(body, "errorDetail", script.ErrorDetail)
	return body
}

func overrideField(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func defaultCannedBody(stage string, payload map[string]any) map[string]any {
	switch stage {
	case constants.StagePickup:
		return map[string]any{"outcome": "no_action_needed", "tokensUsed": 0, "durationMs": 100, "toolCallsCount": 0}
	case constants.StageBrainstorming:
		return map[string]any{
			"outcome":        "no_action_needed",
			"message":        "No clarifications needed.",
			"messageKind":    "brainstorming_complete",
			"tokensUsed":     500,
			"durationMs":     1000,
			"toolCallsCount": 0,
		}
	case constants.StageDesign:
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub design proposal.",
			"messageKind":    "design",
			"tokensUsed":     2000,
			"durationMs":     3000,
			"toolCallsCount": 2,
		}
	case constants.StageImplementationPlan:
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub implementation plan.",
			"messageKind":    "implementation_plan",
			"tokensUsed":     3000,
			"durationMs":     4000,
			"toolCallsCount": 3,
		}
	case constants.StageImplementation:
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub push to PR.",
			"messageKind":    "pull_request_pushed",
			"prUrl":          "https://github.com/example/repo/pull/1",
			"branchName":     "stub/agent-run",
			"tokensUsed":     8000,
			"durationMs":     12000,
			"toolCallsCount": 14,
		}
	case constants.StageChat:
		return chatCannedBody(payload)
	case constants.StageReleaseNotes:
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub release notes proposal.",
			"tokensUsed":     1500,
			"durationMs":     2000,
			"toolCallsCount": 1,
		}
	default:
		return map[string]any{"outcome": "no_action_needed"}
	}
}

// chatCannedBody answers a chat turn: a reply echoing the owner's last message, or a task draft when the turn asks for one.
func chatCannedBody(payload map[string]any) map[string]any {
	bundle, _ := payload["contextBundle"].(map[string]any)
	chat, _ := bundle["chat"].(map[string]any)
	body := map[string]any{"outcome": "output_submitted", "tokensUsed": 300, "durationMs": 800, "toolCallsCount": 0}
	if mode, _ := chat["mode"].(string); mode == constants.ChatModeTaskDraft {
		body["messageKind"] = constants.ChatMessageKindTaskDraft
		title := "Stub task draft"
		if quoted, _ := chat["draftFromMessage"].(map[string]any); quoted != nil {
			if text, _ := quoted["body"].(string); strings.TrimSpace(text) != "" {
				title = "Stub task draft: " + quotedTitle(text)
			}
		}
		body["message"] = "# " + title + "\n\nDescribe the work the conversation agreed on here."
		return body
	}
	body["messageKind"] = constants.ChatMessageKindReply
	body["message"] = "Stub reply: " + lastUserMessage(bundle)
	return body
}

// quotedTitle is the first non-empty line of a quoted answer without its Markdown heading marks,
// cut to a short title.
func quotedTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line == "" {
			continue
		}
		if runes := []rune(line); len(runes) > 80 {
			return string(runes[:80])
		}
		return line
	}
	return ""
}

// lastUserMessage returns the newest owner message of the conversation thread, oldest first.
func lastUserMessage(bundle map[string]any) string {
	thread, _ := bundle["reviewThread"].([]any)
	for index := len(thread) - 1; index >= 0; index-- {
		message, _ := thread[index].(map[string]any)
		if message["author"] == constants.ConversationAuthorUser {
			text, _ := message["body"].(string)
			return text
		}
	}
	return ""
}

// completionTarget is the identity a stage completes under; a chat turn uses its run token.
func completionTarget(target *configuredAgent, payload map[string]any) *configuredAgent {
	runToken, _ := payload["runToken"].(string)
	if runToken == "" {
		return target
	}
	chatTarget := *target
	chatTarget.apiKey = runToken
	return &chatTarget
}
