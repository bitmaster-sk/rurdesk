package test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const (
	chatModelOpus   = "anthropic/claude-opus"
	chatModelSonnet = "anthropic/claude-sonnet"
	chatModelLlama  = "ollama/llama3.3"
)

// offerModels makes the suite agent's gateway report models, with modelDefault as its default.
func (s *AgentConversationApiSuite) offerModels(modelDefault string, models ...string) {
	s.T().Helper()
	ctx := context.Background()
	gatewayRepo := injector.GetAgentGatewayRepository()
	s.Require().NoError(gatewayRepo.SetModels(ctx, s.Agent, model.GatewayRecoveredReq{Models: models, ModelDefault: modelDefault}))
	s.T().Cleanup(func() {
		s.Require().NoError(gatewayRepo.SetModels(ctx, s.Agent, model.GatewayRecoveredReq{Models: []string{}}))
	})
}

func (s *AgentConversationApiSuite) createWith(body string) *model.AgentConversation {
	s.T().Helper()
	conv := &model.AgentConversation{}
	s.call("POST", "/api/private/agent/conversation", body, s.OwnerToken, http.StatusOK, conv)
	return conv
}

func (s *AgentConversationApiSuite) putSettings(conv *model.AgentConversation, body string, want int) {
	s.T().Helper()
	s.call("PUT", s.url(conv.IdConversation, "/settings"), body, s.OwnerToken, want, nil)
}

func (s *AgentConversationApiSuite) settingsOf(conv *model.AgentConversation) *string {
	s.T().Helper()
	return s.detail(conv).Conversation.Model
}

// completeErrored ends the running turn so the agent is free again.
func (s *AgentConversationApiSuite) completeErrored(event *chatEvent) {
	s.T().Helper()
	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", event.Payload.IdTask), chatErroredBody, s.AgentKey, http.StatusOK, nil)
}

func (s *AgentConversationApiSuite) TestCreate_ModelValidatedAgainstGateway() {
	s.offerModels(chatModelSonnet, chatModelOpus, chatModelSonnet, chatModelLlama)
	create := func(fields string) string {
		return fmt.Sprintf(`{"idUserAgent":%d,"idProject":%d,%s}`, s.Agent, s.P1, fields)
	}
	s.Equal("AGENT_MODEL_NOT_OFFERED", s.errCode("POST", "/api/private/agent/conversation",
		create(`"model":"anthropic/claude-haiku"`), s.OwnerToken, http.StatusUnprocessableEntity))
	s.Empty(s.list(""), "a refused create stores nothing")

	chosen := s.createWith(create(`"model":"anthropic/claude-opus"`))
	s.Equal(ptr(chatModelOpus), chosen.Model)

	defaults := s.createWith(create(`"model":""`))
	s.Nil(defaults.Model, "empty means the gateway default")

	llama := s.createWith(create(`"model":"ollama/llama3.3"`))
	s.Equal(ptr(chatModelLlama), llama.Model)

	detail := s.detail(chosen)
	s.Equal(ptr(chatModelOpus), detail.Conversation.Model)
	s.Equal([]string{chatModelOpus, chatModelSonnet, chatModelLlama}, detail.AgentModels.Models)
	s.Equal(ptr(chatModelSonnet), detail.AgentModels.ModelDefault)

	var agents []*model.AgentConversationAgent
	s.call("GET", fmt.Sprintf("/api/private/agent/conversation/agents?idProject=%d", s.P1), "", s.OwnerToken, http.StatusOK, &agents)
	s.Require().Len(agents, 1)
	s.Equal(s.Agent, agents[0].IdUser)
	s.Equal([]string{chatModelOpus, chatModelSonnet, chatModelLlama}, agents[0].Models)
	s.Equal(ptr(chatModelSonnet), agents[0].ModelDefault)
}

func (s *AgentConversationApiSuite) TestChangeSettings_RefusedWhileAnswering_AllowedWhenIdleOrQueued() {
	s.offerModels(chatModelSonnet, chatModelOpus, chatModelSonnet, chatModelLlama)
	conv := s.create(ptr(s.P1), s.OwnerToken)

	s.putSettings(conv, `{"model":"anthropic/claude-opus"}`, http.StatusNoContent)
	s.Equal(ptr(chatModelOpus), s.settingsOf(conv), "idle")

	s.post(conv, "where is the worktree removed?", s.OwnerToken)
	s.Equal("queued", s.detail(conv).AgentStatus.State)
	s.putSettings(conv, `{"model":"anthropic/claude-sonnet"}`, http.StatusNoContent)

	s.tick()
	turn := s.nextEvent("stage_execute")
	s.Equal(ptr(chatModelSonnet), turn.Payload.Model, "the queued turn runs with the model at dispatch")

	s.Equal("AGENT_CONVERSATION_BUSY", s.errCode("PUT", s.url(conv.IdConversation, "/settings"),
		`{"model":"anthropic/claude-opus"}`, s.OwnerToken, http.StatusConflict))
	s.putSettings(conv, `{"model":"anthropic/claude-sonnet"}`, http.StatusNoContent)
	s.Equal(ptr(chatModelSonnet), s.settingsOf(conv), "nothing changes while the agent answers")

	s.completeErrored(turn)
	s.putSettings(conv, `{"model":null}`, http.StatusNoContent)
	s.Nil(s.settingsOf(conv), "idle again after the turn")

	s.Equal("AGENT_MODEL_NOT_OFFERED", s.errCode("PUT", s.url(conv.IdConversation, "/settings"),
		`{"model":"anthropic/claude-haiku"}`, s.OwnerToken, http.StatusUnprocessableEntity))
	s.Equal("NOT_FOUND", s.errCode("PUT", s.url(conv.IdConversation, "/settings"), `{"model":null}`,
		s.OtherToken, http.StatusNotFound))
}

func (s *AgentConversationApiSuite) TestDispatch_SendsConversationModelWithoutEffort() {
	s.offerModels(chatModelSonnet, chatModelOpus, chatModelSonnet, chatModelLlama)
	tasks := injector.GetAgentTaskRepository()
	chosen := s.createWith(fmt.Sprintf(`{"idUserAgent":%d,"idProject":%d,"model":"anthropic/claude-opus","effort":"high"}`, s.Agent, s.P1))
	s.post(chosen, "which model answers?", s.OwnerToken)
	s.tick()
	turn := s.nextEvent("stage_execute")
	s.Equal(ptr(chatModelOpus), turn.Payload.Model)
	s.Nil(turn.Payload.Effort, "a chat turn never sends an effort, even for a model with thinking and a body that names one")
	task, err := tasks.LoadById(context.Background(), turn.Payload.IdTask)
	s.Require().NoError(err)
	s.Equal(ptr(chatModelOpus), task.Model, "the turn records the model it ran on")
	s.Nil(task.Effort)
	s.completeErrored(turn)

	defaults := s.create(ptr(s.P1), s.OwnerToken)
	s.post(defaults, "and here?", s.OwnerToken)
	s.tick()
	turn = s.nextEvent("stage_execute")
	s.Nil(turn.Payload.Model, "the gateway applies its default")
	s.Nil(turn.Payload.Effort)
	task, err = tasks.LoadById(context.Background(), turn.Payload.IdTask)
	s.Require().NoError(err)
	s.Equal(ptr(chatModelSonnet), task.Model, "the gateway default is recorded like an issue stage")
	s.Nil(task.Effort)
}

func (s *AgentConversationApiSuite) TestDispatch_OmitsAModelTheGatewayNoLongerOffers() {
	s.offerModels(chatModelSonnet, chatModelOpus, chatModelSonnet, chatModelLlama)
	conv := s.createWith(fmt.Sprintf(`{"idUserAgent":%d,"idProject":%d,"model":"anthropic/claude-opus"}`, s.Agent, s.P1))
	s.offerModels(chatModelLlama, chatModelLlama)

	s.post(conv, "still there?", s.OwnerToken)
	s.tick()
	turn := s.nextEvent("stage_execute")

	s.Nil(turn.Payload.Model, "a dropped model falls back to the gateway default")
	s.Nil(turn.Payload.Effort)
	task, err := injector.GetAgentTaskRepository().LoadById(context.Background(), turn.Payload.IdTask)
	s.Require().NoError(err)
	s.Equal(ptr(chatModelLlama), task.Model)
	s.Nil(task.Effort)
}
