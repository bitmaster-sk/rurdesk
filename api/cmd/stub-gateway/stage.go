package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
	Stall bool `json:"stall"`
}

func executeStage(target *configuredAgent, idRun int64, payload map[string]any) {
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

	if err := sendComplete(ctx, target, idTask, stagedCannedBody(target, stage)); err != nil {
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
func stagedCannedBody(target *configuredAgent, stage string) map[string]any {
	body := defaultCannedBody(stage)
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

func defaultCannedBody(stage string) map[string]any {
	switch stage {
	case "pickup":
		return map[string]any{"outcome": "no_action_needed", "tokensUsed": 0, "durationMs": 100, "toolCallsCount": 0}
	case "brainstorming":
		return map[string]any{
			"outcome":        "no_action_needed",
			"message":        "No clarifications needed.",
			"messageKind":    "brainstorming_complete",
			"tokensUsed":     500,
			"durationMs":     1000,
			"toolCallsCount": 0,
		}
	case "design":
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub design proposal.",
			"messageKind":    "design",
			"tokensUsed":     2000,
			"durationMs":     3000,
			"toolCallsCount": 2,
		}
	case "implementation_plan":
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub implementation plan.",
			"messageKind":    "implementation_plan",
			"tokensUsed":     3000,
			"durationMs":     4000,
			"toolCallsCount": 3,
		}
	case "implementation":
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
	default:
		return map[string]any{"outcome": "no_action_needed"}
	}
}
