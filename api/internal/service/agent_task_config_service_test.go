package service

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildAgentTaskConfig(t *testing.T) {
	taskConfigService := NewAgentTaskConfigService(NewAppSettingsService(nil), nil)
	modelDefault := "openai/kimi"
	agentPlanTurns := 120
	gatewayLimit := 300
	gateway := &model.AgentGateway{Models: []string{"openai/kimi", "ollama/qwen3:8b"}, ModelDefault: &modelDefault, MaxTurnsPlan: &agentPlanTurns}
	plan := model.AgentStagePlan{
		Model: "ollama/qwen3:8b",
		Stages: []model.AgentStagePlanEntry{
			{Name: constants.StageDesign, Model: "openai/kimi", MaxTurns: 40},
			{Name: constants.StageBrainstorming},
			{Name: constants.StageImplementation, Model: "anthropic/opus"},
		},
	}

	tests := []struct {
		name         string
		plan         model.AgentStagePlan
		stage        string
		gateway      *model.AgentGateway
		wantModel    *string
		wantTurns    int
		wantFallback *string
	}{
		{"the stage choice wins", plan, constants.StageDesign, gateway, stringPtr("openai/kimi"), 40, nil},
		{"the run choice covers a stage without one", plan, constants.StageBrainstorming, gateway, stringPtr("ollama/qwen3:8b"), 120, nil},
		{"a model the gateway dropped falls back to its default", plan, constants.StageImplementation, gateway, stringPtr("openai/kimi"), 500, stringPtr("anthropic/opus")},
		{"no choice runs the gateway default", model.AgentStagePlan{}, constants.StageDesign, gateway, stringPtr("openai/kimi"), 120, nil},
		{"an agent that never reported leaves the model to the gateway", plan, constants.StageDesign, &model.AgentGateway{}, nil, 40, nil},
		{"turns are capped at the gateway limit", plan, constants.StageImplementation, &model.AgentGateway{MaxTurnsLimit: &gatewayLimit}, nil, 300, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskConfig := taskConfigService.BuildAgentTaskConfig(tt.plan, tt.stage, tt.gateway)

			assert.Equal(t, tt.wantModel, taskConfig.Model)
			assert.Equal(t, tt.wantTurns, taskConfig.MaxTurns)
			assert.Equal(t, tt.wantFallback, taskConfig.FallbackFrom)
		})
	}
}

func stringPtr(value string) *string {
	return &value
}
