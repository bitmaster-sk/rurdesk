package service

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAgentTaskConfig(t *testing.T) {
	taskConfigService := NewAgentTaskConfigService(NewAppSettingsService(nil), nil, nil)
	modelDefault := "openai/kimi"
	agentDesignTurns := 120
	gatewayLimit := 300
	gateway := &model.AgentGateway{
		Models: []string{"openai/kimi", "ollama/qwen3:8b"}, ModelDefault: &modelDefault,
		StageDefaults: map[string]model.AgentStageOptions{constants.StageDesign: {MaxTurns: &agentDesignTurns}},
	}
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
		{"the run choice covers a stage without one", plan, constants.StageBrainstorming, gateway, stringPtr("ollama/qwen3:8b"), 250, nil},
		{"a model the gateway dropped falls back to its default", plan, constants.StageImplementation, gateway, stringPtr("openai/kimi"), 500, stringPtr("anthropic/opus")},
		{"no choice runs the gateway default", model.AgentStagePlan{}, constants.StageDesign, gateway, stringPtr("openai/kimi"), 120, nil},
		{"each stage has its own default turn limit", model.AgentStagePlan{}, constants.StageReleaseNotes, gateway, stringPtr("openai/kimi"), 250, nil},
		{"an agent that never reported leaves the model to the gateway", plan, constants.StageDesign, &model.AgentGateway{}, nil, 40, nil},
		{"turns are capped at the gateway limit", plan, constants.StageImplementation, &model.AgentGateway{MaxTurnsLimit: &gatewayLimit}, nil, 300, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskConfig := taskConfigService.BuildAgentTaskConfig(tt.plan, tt.stage, tt.gateway, nil)

			assert.Equal(t, tt.wantModel, taskConfig.Model)
			assert.Equal(t, tt.wantTurns, taskConfig.MaxTurns)
			assert.Equal(t, tt.wantFallback, taskConfig.FallbackFrom)
		})
	}
}

func TestBuildAgentTaskConfigEffort(t *testing.T) {
	store := &fakeStore{data: map[string]string{constants.SettingAgentEffort(constants.StageImplementation): `"medium"`}}
	settings := NewAppSettingsService(store)
	require.NoError(t, settings.Load(context.Background()))
	taskConfigService := NewAgentTaskConfigService(settings, nil, nil)
	modelDefault := "openai/kimi"
	agentBrainstormingEffort := constants.EffortHigh
	gateway := &model.AgentGateway{
		Models: []string{"openai/kimi", "openai/gpt-4o"}, ModelDefault: &modelDefault,
		StageDefaults: map[string]model.AgentStageOptions{constants.StageBrainstorming: {Effort: &agentBrainstormingEffort}},
	}
	withoutThinking := []string{"openai/gpt-4o"}

	tests := []struct {
		name       string
		plan       model.AgentStagePlan
		stage      string
		gateway    *model.AgentGateway
		wantEffort *string
	}{
		{"the stage choice wins", model.AgentStagePlan{Effort: "low", Stages: []model.AgentStagePlanEntry{{Name: constants.StageDesign, Effort: "max"}}}, constants.StageDesign, gateway, stringPtr("max")},
		{"the run choice covers a stage without one", model.AgentStagePlan{Effort: "low"}, constants.StageImplementation, gateway, stringPtr("low")},
		{"a stage takes the agent default", model.AgentStagePlan{}, constants.StageBrainstorming, gateway, stringPtr("high")},
		{"the agent default of one stage does not reach another", model.AgentStagePlan{}, constants.StageDesign, gateway, nil},
		{"implementation takes the settings default when the agent has none", model.AgentStagePlan{}, constants.StageImplementation, gateway, stringPtr("medium")},
		{"no default anywhere leaves it to the model", model.AgentStagePlan{}, constants.StageDesign, &model.AgentGateway{Models: []string{"openai/kimi"}}, nil},
		{"a model without thinking gets no effort", model.AgentStagePlan{Model: "openai/gpt-4o", Effort: "max"}, constants.StageDesign, gateway, nil},
		{"an agent that never reported models still gets the effort", model.AgentStagePlan{}, constants.StageImplementation, &model.AgentGateway{}, stringPtr("medium")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskConfig := taskConfigService.BuildAgentTaskConfig(tt.plan, tt.stage, tt.gateway, withoutThinking)

			assert.Equal(t, tt.wantEffort, taskConfig.Effort)
		})
	}
}

func TestGetStageDefaultsNamesTheSource(t *testing.T) {
	store := &fakeStore{data: map[string]string{
		constants.SettingAgentEffort(constants.StageDesign):         `"low"`,
		constants.SettingAgentMaxTurns(constants.StageReleaseNotes): "60",
	}}
	settings := NewAppSettingsService(store)
	require.NoError(t, settings.Load(context.Background()))
	taskConfigService := NewAgentTaskConfigService(settings, nil, nil)
	agentEffort := constants.EffortMax
	agentTurns := 700
	gatewayLimit := 600
	gateway := &model.AgentGateway{
		MaxTurnsLimit: &gatewayLimit,
		StageDefaults: map[string]model.AgentStageOptions{constants.StageImplementation: {MaxTurns: &agentTurns, Effort: &agentEffort}},
	}

	defaults := taskConfigService.GetStageDefaults(gateway)

	assert.Len(t, defaults, len(constants.SkillStages))
	assert.Equal(t, model.AgentStageDefault{MaxTurns: 250, Effort: model.AgentEffortDefault{Effort: stringPtr("low"), Source: constants.EffortSourceSettings}}, defaults[constants.StageDesign])
	assert.Equal(t, model.AgentStageDefault{MaxTurns: 600, Effort: model.AgentEffortDefault{Effort: stringPtr("max"), Source: constants.EffortSourceAgent}}, defaults[constants.StageImplementation])
	assert.Equal(t, model.AgentStageDefault{MaxTurns: 60, Effort: model.AgentEffortDefault{Source: constants.EffortSourceModel}}, defaults[constants.StageReleaseNotes])
}

func TestFilterModelsWithoutThinking(t *testing.T) {
	models := []string{"openai/GPT-4o", "openai/gpt-5", "ollama/llama3.3", "ollama/qwen3:8b"}

	without := FilterModelsWithoutThinking(models, []string{"gpt-4o"}, []string{"ollama/llama3.3"})

	assert.Equal(t, []string{"openai/GPT-4o", "ollama/llama3.3"}, without)
}

func TestModelName(t *testing.T) {
	assert.Equal(t, "gpt-4o", ModelName("openrouter/openai/GPT-4o"))
	assert.Equal(t, "claude-opus-5-5", ModelName("claude-opus-5-5"))
}

func stringPtr(value string) *string {
	return &value
}
