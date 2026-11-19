package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// AgentConversationController serves the owner's chats with agents; every conversation route
// answers 404 for another user's conversation. Create and AttachProject answer 422
// AGENT_NOT_IN_PROJECT when the agent or owner lacks the project, AvailableAgents 403 AGENT_CONVERSATION_NO_ACCESS.
type AgentConversationController struct {
	conversations *service.AgentConversationService
	dispatcher    *agent.Dispatcher
}

func NewAgentConversationController(
	conversations *service.AgentConversationService,
	dispatcher *agent.Dispatcher,
) *AgentConversationController {
	return &AgentConversationController{conversations: conversations, dispatcher: dispatcher}
}

func (ctrl *AgentConversationController) GetConversations(c *gin.Context) {
	idProject, ok := optionalQueryId(c, "idProject")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	conversations, err := ctrl.conversations.LoadConversations(ctx, &owner, idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, conversations)
}

func (ctrl *AgentConversationController) CreateConversation(c *gin.Context) {
	var req model.CreateAgentConversationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	conversation, err := ctrl.conversations.CreateConversation(ctx, &owner, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, conversation)
}

func (ctrl *AgentConversationController) GetConversation(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	detail, err := ctrl.conversations.LoadDetail(ctx, &owner, idConversation)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, detail)
}

// DeleteConversation removes the conversation, then tells the gateway to stop a running turn and remove the
// run's worktree; a failed delivery does not undo the delete.
func (ctrl *AgentConversationController) DeleteConversation(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	run, err := ctrl.conversations.DeleteConversation(ctx, &owner, idConversation)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if run != nil {
		go func() {
			if err := ctrl.dispatcher.DispatchRunDiscarded(context.Background(), run); err != nil {
				log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("conversation delete: failed to tell the gateway to discard the run")
			}
		}()
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *AgentConversationController) GetMessages(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	messages, err := ctrl.conversations.LoadMessages(ctx, &owner, idConversation)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, messages)
}

func (ctrl *AgentConversationController) CreateMessage(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	var req model.CreateAgentConversationMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	message, err := ctrl.conversations.CreateMessage(ctx, &owner, idConversation, req.Message)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, message)
}

func (ctrl *AgentConversationController) AddProject(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	var req model.AddAgentConversationProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.AddProject(ctx, &owner, idConversation, req.IdProject); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *AgentConversationController) RemoveProject(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.RemoveProject(ctx, &owner, idConversation, idProject); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *AgentConversationController) SetRead(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.SetRead(ctx, &owner, idConversation); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetPinned pins or unpins the owner's conversation.
func (ctrl *AgentConversationController) SetPinned(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	var req model.SetAgentConversationPinnedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.SetPinned(ctx, &owner, idConversation, *req.IsPinned); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateSettings replaces the model; 409 AGENT_CONVERSATION_BUSY while the agent answers,
// 422 for a model the gateway does not offer.
func (ctrl *AgentConversationController) UpdateSettings(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	var req model.UpdateAgentConversationSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.UpdateSettings(ctx, &owner, idConversation, req); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *AgentConversationController) CreateTaskDraft(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	var req model.CreateAgentConversationTaskDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.CreateTaskDraft(ctx, &owner, idConversation, req.IdMessage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Retry re-queues the turn that ended in error; 409 when there is none to retry.
func (ctrl *AgentConversationController) Retry(c *gin.Context) {
	idConversation, ok := conversationId(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	if err := ctrl.conversations.Retry(ctx, &owner, idConversation); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *AgentConversationController) GetAgents(c *gin.Context) {
	idProject, ok := optionalQueryId(c, "idProject")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	agents, err := ctrl.conversations.LoadAgents(ctx, &owner, idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, agents)
}

func (ctrl *AgentConversationController) GetUnreadCount(c *gin.Context) {
	ctx := c.Request.Context()
	owner, _ := extctx.GetUser(ctx)
	count, err := ctrl.conversations.CountUnread(ctx, &owner)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, model.AgentConversationUnreadCountRes{Count: count})
}

// conversationId parses :id, writing a 400 when it is not a number.
func conversationId(c *gin.Context) (int64, bool) {
	idConversation, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return 0, false
	}
	return idConversation, true
}

// optionalQueryId parses an optional numeric query parameter, writing a 400 when
// it is present but not a number.
func optionalQueryId(c *gin.Context, name string) (*int64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return nil, false
	}
	return &parsed, true
}
