package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/rs/zerolog/log"
)

// buildChatPayload builds the turn's stage_execute payload. The conversation's main project goes with
// its repository when it has a git integration; a chat without one runs in a scratch directory.
func (d *Dispatcher) buildChatPayload(ctx context.Context, run *model.AgentRun, task *model.AgentTask, runToken string) (map[string]any, error) {
	conversation, err := d.conversationRepo.LoadConversationByRun(ctx, run.IdRun)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("chat run %d has no conversation", run.IdRun)
	}
	if conversation.IdProject != nil && run.IdProject != *conversation.IdProject {
		if err := d.agentRunRepo.SetChatProject(ctx, run.IdRun, *conversation.IdProject); err != nil {
			return nil, err
		}
		run.IdProject = *conversation.IdProject
	}
	mode := constants.ChatModeReply
	if task.ChatMode != nil {
		mode = *task.ChatMode
	}
	agentUser, err := d.userRepo.LoadUser(ctx, run.IdUserAgent)
	if err != nil {
		return nil, fmt.Errorf("loading agent user: %w", err)
	}
	since := task.CreatedAt
	if task.StartedAt != nil {
		since = *task.StartedAt
	}
	messages, err := d.conversationRepo.LoadMessagesUntil(ctx, conversation.IdConversation, since)
	if err != nil {
		return nil, err
	}
	thread, isTruncated, err := chatThread(messages, constants.ChatHistoryMaxBytes)
	if err != nil {
		return nil, err
	}
	projects, err := d.chatProjects(ctx, conversation)
	if err != nil {
		return nil, err
	}

	chat := map[string]any{
		"idConversation": conversation.IdConversation,
		"mode":           mode,
		"projects":       projects,
		"headCommit":     conversation.HeadCommit,
	}
	if mode == constants.ChatModeTaskDraft && conversation.IdTaskDraftMessage != nil {
		quoted, err := d.conversationRepo.LoadMessage(ctx, conversation.IdConversation, *conversation.IdTaskDraftMessage)
		if err != nil {
			return nil, err
		}
		if quoted != nil {
			chat["draftFromMessage"] = map[string]any{"idMessage": quoted.IdMessage, "body": quoted.Body}
		}
	}
	bundle := map[string]any{
		"agent":            agentUser,
		"reviewThread":     thread,
		"historyTruncated": isTruncated,
		"chat":             chat,
	}
	payload := map[string]any{
		"idTask":        task.IdTask,
		"stage":         task.Stage,
		"attemptNo":     task.AttemptNo,
		"stagePlan":     run.StagePlan,
		"contextBundle": bundle,
		"runToken":      runToken,
	}
	chatModel, err := d.chatModel(ctx, run, task, conversation)
	if err != nil {
		return nil, err
	}
	if chatModel != nil {
		payload["model"] = *chatModel
	}
	if conversation.IdProject == nil {
		return payload, nil
	}
	project, err := d.projectRepo.LoadProject(ctx, *conversation.IdProject)
	if err != nil {
		return nil, fmt.Errorf("loading project: %w", err)
	}
	bundle["project"] = project
	repo, err := d.agentRunGitRepo(ctx, run)
	if errors.Is(err, ErrProjectHasNoGitIntegration) {
		return payload, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolving the chat repo: %w", err)
	}
	payload["repo"] = repo
	return payload, nil
}

// chatProject is a project the turn may read; IsMain marks the one whose
// repository the agent sees.
type chatProject struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name"`
	IsMain    bool   `json:"isMain"`
}

// chatModel returns the conversation's model as it is now, nil leaving the default to the gateway,
// and records on the task what the turn runs with, the gateway default model included, like an issue
// stage. A model the gateway no longer offers is left out. A chat turn sends no effort.
func (d *Dispatcher) chatModel(ctx context.Context, run *model.AgentRun, task *model.AgentTask, conversation *model.AgentConversation) (*string, error) {
	gateway, err := d.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil {
		return nil, fmt.Errorf("loading gateway: %w", err)
	}
	if gateway == nil {
		return nil, fmt.Errorf("no gateway configured for agent %d", run.IdUserAgent)
	}
	chatModel := conversation.Model
	if chatModel != nil && !gateway.IsSupportedModel(*chatModel) {
		log.Warn().Int64("idTask", task.IdTask).Str("requested", *chatModel).Msg("chat model no longer offered by the gateway, falling back to its default")
		chatModel = nil
	}
	usedModel := gateway.ModelDefault
	if chatModel != nil {
		usedModel = chatModel
	}
	if err := d.taskRepo.SetModel(ctx, task.IdTask, usedModel); err != nil {
		return nil, err
	}
	task.Model, task.Effort = usedModel, nil
	return chatModel, nil
}

// chatProjects lists the projects the turn may read, recomputed now.
func (d *Dispatcher) chatProjects(ctx context.Context, conversation *model.AgentConversation) ([]chatProject, error) {
	idsProject, err := d.conversations.LoadReadableProjects(ctx, conversation)
	if err != nil {
		return nil, err
	}
	projects := make([]chatProject, 0, len(idsProject))
	for _, idProject := range idsProject {
		project, err := d.projectRepo.LoadProject(ctx, idProject)
		if err != nil {
			return nil, fmt.Errorf("loading project %d: %w", idProject, err)
		}
		projects = append(projects, chatProject{
			IdProject: idProject,
			Name:      project.Name,
			IsMain:    conversation.IdProject != nil && *conversation.IdProject == idProject,
		})
	}
	return projects, nil
}

// chatThread keeps the newest messages whose JSON fits in maxBytes, oldest first, and reports
// whether older ones were dropped. A body over ChatMessageMaxBytes is cut with a marker first;
// entries are pre-encoded so the budget counts what the gateway receives.
func chatThread(messages []*model.AgentConversationMessage, maxBytes int) ([]json.RawMessage, bool, error) {
	kept := make([]json.RawMessage, 0, len(messages))
	size := 0
	for i := len(messages) - 1; i >= 0; i-- {
		encoded, err := json.Marshal(capChatMessage(messages[i], constants.ChatMessageMaxBytes))
		if err != nil {
			return nil, false, fmt.Errorf("encoding conversation message %d: %w", messages[i].IdMessage, err)
		}
		if size+len(encoded) > maxBytes {
			slices.Reverse(kept)
			return kept, true, nil
		}
		size += len(encoded)
		kept = append(kept, encoded)
	}
	slices.Reverse(kept)
	return kept, false, nil
}

// capChatMessage returns the message with its body cut to maxBytes on a rune boundary plus a
// marker saying how much was left out. The original is not modified.
func capChatMessage(message *model.AgentConversationMessage, maxBytes int) *model.AgentConversationMessage {
	if len(message.Body) <= maxBytes {
		return message
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(message.Body[cut]) {
		cut--
	}
	capped := *message
	capped.Body = fmt.Sprintf("%s\n\n[message truncated: %d of %d bytes omitted]",
		message.Body[:cut], len(message.Body)-cut, len(message.Body))
	return &capped
}
