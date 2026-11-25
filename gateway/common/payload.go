package common

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/gateway/model"
)

func parseStageExecutePayload(event WebhookEvent) (Task, error) {
	if event.IdRun == 0 {
		return Task{}, fmt.Errorf("missing idRun")
	}
	if len(event.Payload) == 0 {
		return Task{}, fmt.Errorf("empty payload")
	}

	var payload model.StageExecutePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return Task{}, fmt.Errorf("decoding stage_execute payload: %w", err)
	}

	task := Task{
		IdRun:          event.IdRun,
		IdIssue:        event.IdIssue,
		IdUserAgent:    event.IdUserAgent,
		IdProject:      event.IdProject,
		IdTask:         payload.IdTask,
		Stage:          payload.Stage,
		AttemptNo:      payload.AttemptNo,
		PrBranch:       payload.PrBranch,
		RequestedModel: payload.Model,
		MaxTurns:       payload.MaxTurns,
		Effort:         payload.Effort,
	}

	if payload.Repo != nil {
		task.Repo = AgentRunGitRepo{
			CloneUrl:       payload.Repo.CloneUrl,
			Host:           payload.Repo.Host,
			RepoPath:       payload.Repo.RepoPath,
			BaseBranch:     payload.Repo.BaseBranch,
			EncryptedToken: payload.Repo.EncryptedToken,
		}
	}

	if bundle := payload.ContextBundle; bundle != nil {
		if bundle.Issue != nil {
			task.IdIssuePublic = bundle.Issue.IdIssuePublic
			task.IssueTitle = bundle.Issue.Title
			task.IssueDesc = bundle.Issue.Description
		}
		task.ApprovedDesign = messageBody(bundle.ApprovedDesign)
		task.ApprovedImplPlan = messageBody(bundle.ApprovedImplPlan)
		task.RejectedOutput = messageBody(bundle.RejectedOutput)
		if bundle.ApprovedMockupRef != nil {
			task.ApprovedMockupRef = *bundle.ApprovedMockupRef
		}
		task.IsShortWorkflow = bundle.IsShortWorkflow
		task.ResumedAfter = bundle.ResumedAfter
		task.IsRevision = bundle.IsRevision
		for _, skill := range decodeEach[model.Skill](bundle.Skills) {
			if skill.Name == "" || skill.Content == "" {
				continue
			}
			task.Skills = append(task.Skills, Skill(skill))
		}
		task.Wiki = parseWiki(bundle.Wiki)
		task.WikiProposals = parseWikiProposals(bundle.WikiProposals)
		task.Release = parseRelease(bundle.Release)
		if payload.Stage == StageChat {
			task.Chat = parseChat(bundle)
			task.Conversation = parseChatThread(bundle.ReviewThread)
		} else {
			task.Conversation = parseIssueThread(bundle.ReviewThread, bundle.IdMessageSeen)
			task.HasNewFeedback = slices.ContainsFunc(task.Conversation, func(message TaskMessage) bool { return message.IsNew })
		}
	}

	if task.Stage == StageChat {
		task.RunToken = payload.RunToken
		if task.Chat == nil {
			task.Chat = &ChatContext{Mode: ChatModeReply}
		}
	}

	if task.IdTask == 0 {
		return Task{}, fmt.Errorf("missing idTask")
	}
	if task.Stage == "" {
		return Task{}, fmt.Errorf("missing stage")
	}
	return task, nil
}

// parseIssueThread marks the people's comments above idMessageSeen as new: the feedback the previous
// attempt of the stage has not seen. Nothing is marked without idMessageSeen.
func parseIssueThread(rawMessages []json.RawMessage, idMessageSeen *int64) []TaskMessage {
	var conversation []TaskMessage
	for _, message := range decodeEach[model.Message](rawMessages) {
		creatorName := ""
		isAgent := false
		if message.Creator != nil {
			creatorName = message.Creator.Name
			isAgent = message.Creator.IsAgent
		}
		conversation = append(conversation, TaskMessage{
			CreatorName: creatorName,
			CreatedAt:   message.CreatedAt,
			Kind:        message.MessageKind,
			Message:     conversationBody(message.MessageKind, message.Message),
			Reference:   messageReference(message),
			IsNew: idMessageSeen != nil && !isAgent && message.MessageKind == MessageKindComment &&
				message.IdMessage > *idMessageSeen,
		})
	}
	return conversation
}

// messageReference gives the agent the message id and, for an anchored comment, the text it is
// about: the selected quote, or the anchored source lines when the whole range was meant.
func messageReference(message model.Message) string {
	if message.IdMessage == 0 {
		return ""
	}
	reference := fmt.Sprintf(" · #%d", message.IdMessage)
	if message.Anchor == nil {
		return reference
	}
	anchoredText := message.Anchor.AnchorText
	if message.Anchor.Quote != nil {
		anchoredText = *message.Anchor.Quote
	}
	reference += fmt.Sprintf(" · on #%d %q", message.Anchor.IdParentMessage, strings.Join(strings.Fields(anchoredText), " "))
	if message.Anchor.IsOutdated {
		reference += " (that text has changed since)"
	}
	return reference
}

// parseChatThread reads conversation messages; their author (user, agent or
// system) stands in for the creator name.
func parseChatThread(rawMessages []json.RawMessage) []TaskMessage {
	var conversation []TaskMessage
	for _, message := range decodeEach[model.ChatMessage](rawMessages) {
		conversation = append(conversation, TaskMessage{
			CreatorName: message.Author,
			CreatedAt:   message.CreatedAt,
			Kind:        message.Kind,
			Message:     message.Body,
		})
	}
	return conversation
}

// decodeEach skips elements that do not decode so one malformed entry never fails a stage.
func decodeEach[T any](rawItems []json.RawMessage) []T {
	var items []T
	for _, rawItem := range rawItems {
		var item T
		if err := json.Unmarshal(rawItem, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func messageBody(message *model.Message) string {
	if message == nil {
		return ""
	}
	return message.Message
}

// conversationBody drops the body of artifacts the prompt already renders in
// full sections of their own, keeping only their place in the thread.
func conversationBody(kind, message string) string {
	switch kind {
	case MessageKindDesign:
		return "(design document — body omitted here)"
	case MessageKindImplementationPlan:
		return "(implementation plan — body omitted here)"
	default:
		return message
	}
}

func parseWiki(promptContext *model.WikiPromptContext) *WikiContext {
	if promptContext == nil {
		return nil
	}
	wiki := &WikiContext{IndexMore: promptContext.IndexMore}
	for _, page := range decodeEach[model.WikiPromptPage](promptContext.Pages) {
		if page.Slug == "" || page.Body == "" {
			continue
		}
		wiki.Pages = append(wiki.Pages, WikiPage(page))
	}
	for _, entry := range decodeEach[model.WikiPromptIndexEntry](promptContext.Index) {
		if entry.Slug == "" {
			continue
		}
		wiki.Index = append(wiki.Index, WikiIndexEntry(entry))
	}
	if len(wiki.Pages) == 0 && len(wiki.Index) == 0 && wiki.IndexMore == 0 {
		return nil
	}
	return wiki
}

// parseChat reads the conversation of a chat turn. An unknown mode is answered
// as an ordinary reply.
func parseChat(bundle *model.ContextBundle) *ChatContext {
	chat := &ChatContext{Mode: ChatModeReply, IsHistoryTruncated: bundle.HistoryTruncated}
	if bundle.Chat != nil {
		chat.IdConversation = bundle.Chat.IdConversation
		chat.HeadCommit = bundle.Chat.HeadCommit
		if bundle.Chat.Mode == ChatModeTaskDraft {
			chat.Mode = ChatModeTaskDraft
			if bundle.Chat.DraftFromMessage != nil {
				source := ChatDraftFromMessage(*bundle.Chat.DraftFromMessage)
				chat.DraftFromMessage = &source
			}
		}
		for _, project := range bundle.Chat.Projects {
			chat.Projects = append(chat.Projects, ChatProject(project))
		}
	}
	return chat
}

func parseRelease(promptContext *model.ReleasePromptContext) *ReleaseContext {
	if promptContext == nil {
		return nil
	}
	release := &ReleaseContext{Name: promptContext.Name, Tag: promptContext.Tag}
	for _, issue := range promptContext.OpenIssues {
		release.OpenIssues = append(release.OpenIssues, ReleaseIssue(issue))
	}
	for _, section := range promptContext.Sections {
		name := ""
		if section.Name != nil {
			name = *section.Name
		}
		issues := make([]ReleaseIssue, len(section.Issues))
		for i, issue := range section.Issues {
			issues[i] = ReleaseIssue(issue)
		}
		release.Sections = append(release.Sections, ReleaseSection{Name: name, Issues: issues})
	}
	return release
}

func parseWikiProposals(rawProposals []json.RawMessage) []WikiProposal {
	var proposals []WikiProposal
	for _, proposal := range decodeEach[model.WikiProposalAgentView](rawProposals) {
		if proposal.Slug == "" {
			continue
		}
		parent := ""
		if proposal.Parent != nil {
			parent = *proposal.Parent
		}
		proposals = append(proposals, WikiProposal{
			Kind:   proposal.Kind,
			Slug:   proposal.Slug,
			Title:  proposal.Title,
			Parent: parent,
			Reason: proposal.Reason,
		})
	}
	return proposals
}
