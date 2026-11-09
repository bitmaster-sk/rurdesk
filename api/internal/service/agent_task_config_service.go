package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type AgentTaskConfig struct {
	Model        *string
	MaxTurns     int
	FallbackFrom *string
	Effort       *string
}

type AgentTaskConfigService struct {
	settings    *AppSettingsService
	agentGwRepo *repository.AgentGatewayRepository
	aiModelRepo *repository.AiModelRepository
}

func NewAgentTaskConfigService(settings *AppSettingsService, agentGwRepo *repository.AgentGatewayRepository, aiModelRepo *repository.AiModelRepository) *AgentTaskConfigService {
	return &AgentTaskConfigService{settings: settings, agentGwRepo: agentGwRepo, aiModelRepo: aiModelRepo}
}

func (s *AgentTaskConfigService) GetStageDefaults(gw *model.AgentGateway) map[string]model.AgentStageDefault {
	defaults := make(map[string]model.AgentStageDefault, len(constants.SkillStages))
	for _, stage := range constants.SkillStages {
		defaults[stage] = s.stageDefault(gw, stage)
	}
	return defaults
}

func (s *AgentTaskConfigService) stageDefault(gw *model.AgentGateway, stage string) model.AgentStageDefault {
	agentDefault := gw.StageDefaults[stage]
	maxTurns := s.settings.AgentMaxTurns(stage)
	if agentDefault.MaxTurns != nil {
		maxTurns = *agentDefault.MaxTurns
	}
	return model.AgentStageDefault{
		MaxTurns: capTurns(maxTurns, gw),
		Effort:   effortDefault(agentDefault.Effort, s.settings.AgentEffort(stage)),
	}
}

func effortDefault(agentEffort *string, settingsEffort string) model.AgentEffortDefault {
	switch {
	case agentEffort != nil:
		return model.AgentEffortDefault{Effort: agentEffort, Source: constants.EffortSourceAgent}
	case settingsEffort != "":
		return model.AgentEffortDefault{Effort: &settingsEffort, Source: constants.EffortSourceSettings}
	}
	return model.AgentEffortDefault{Source: constants.EffortSourceModel}
}

func (s *AgentTaskConfigService) ModelsWithoutThinking(ctx context.Context, gw *model.AgentGateway) ([]string, error) {
	names := make([]string, len(gw.Models))
	for index, id := range gw.Models {
		names[index] = ModelName(id)
	}
	known, err := s.aiModelRepo.LoadNamesWithoutThinking(ctx, names)
	if err != nil {
		return nil, err
	}
	return FilterModelsWithoutThinking(gw.Models, known, gw.ModelsWithoutThinking), nil
}

func FilterModelsWithoutThinking(models, knownNames, reported []string) []string {
	without := []string{}
	for _, id := range models {
		if slices.Contains(knownNames, ModelName(id)) || slices.Contains(reported, id) {
			without = append(without, id)
		}
	}
	return without
}

// Must match ModelsSnapshot.modelName in script/models-snapshot.mjs, which writes the names this is looked up against.
func ModelName(id string) string {
	return strings.ToLower(id[strings.LastIndex(id, "/")+1:])
}

func (s *AgentTaskConfigService) ApplyToOverviews(ctx context.Context, entries []*model.AgentOverview) error {
	idsUserAgent := make([]int64, len(entries))
	for i, entry := range entries {
		idsUserAgent[i] = entry.IdUserAgent
	}
	gatewayByAgent, err := s.agentGwRepo.LoadByAgentUsers(ctx, idsUserAgent)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		gateway, ok := gatewayByAgent[entry.IdUserAgent]
		if !ok {
			gateway = &model.AgentGateway{Models: []string{}}
		}
		entry.Models = gateway.Models
		entry.ModelDefault = gateway.ModelDefault
		entry.MaxTurnsLimit = gateway.MaxTurnsLimit
		entry.DefaultsByStage = s.GetStageDefaults(gateway)
		entry.ModelsWithoutThinking, err = s.ModelsWithoutThinking(ctx, gateway)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentTaskConfigService) ValidateChoices(gateway *model.AgentGateway, choices model.RunChoices) error {
	stages := slices.Collect(maps.Keys(choices.IdsSkillByStage))
	stages = slices.AppendSeq(stages, maps.Keys(choices.ModelByStage))
	stages = slices.AppendSeq(stages, maps.Keys(choices.MaxTurnsByStage))
	stages = slices.AppendSeq(stages, maps.Keys(choices.EffortByStage))
	for _, stage := range stages {
		if !constants.IsSkillStage(stage) {
			return errs.ErrBadRequest.WithMessage("unknown stage: " + stage)
		}
	}
	for stage, maxTurns := range choices.MaxTurnsByStage {
		if maxTurns < 1 || maxTurns > constants.MaxAgentTurns {
			return errs.ErrValidation.WithMessage(fmt.Sprintf("maxTurns of %s must be between 1 and %d", stage, constants.MaxAgentTurns))
		}
	}
	efforts := append(slices.Collect(maps.Values(choices.EffortByStage)), choices.Effort)
	for _, effort := range efforts {
		if effort != "" && !constants.IsEffort(effort) {
			return errs.ErrValidation.WithMessage("effort must be one of " + strings.Join(constants.Efforts, ", "))
		}
	}
	models := append(slices.Collect(maps.Values(choices.ModelByStage)), choices.Model)
	for _, chosen := range models {
		if chosen != "" && !gateway.IsSupportedModel(chosen) {
			return errs.ErrAgentModelNotOffered.WithMessage("the agent does not offer the model " + chosen)
		}
	}
	return nil
}

func (s *AgentTaskConfigService) BuildAgentTaskConfig(plan model.AgentStagePlan, stage string, gw *model.AgentGateway, modelsWithoutThinking []string) AgentTaskConfig {
	entry := stageEntry(plan, stage)
	stageDefault := s.stageDefault(gw, stage)
	taskConfig := AgentTaskConfig{MaxTurns: capTurns(entry.MaxTurns, gw), Effort: resolveEffort(plan, entry, stageDefault)}
	if taskConfig.MaxTurns == 0 {
		taskConfig.MaxTurns = stageDefault.MaxTurns
	}

	if len(gw.Models) == 0 {
		return taskConfig
	}
	requested := entry.Model
	if requested == "" {
		requested = plan.Model
	}
	switch {
	case requested == "":
		taskConfig.Model = gw.ModelDefault
	case gw.IsSupportedModel(requested):
		taskConfig.Model = &requested
	default:
		taskConfig.Model = gw.ModelDefault
		taskConfig.FallbackFrom = &requested
	}
	if taskConfig.Model != nil && slices.Contains(modelsWithoutThinking, *taskConfig.Model) {
		taskConfig.Effort = nil
	}
	return taskConfig
}

func resolveEffort(plan model.AgentStagePlan, entry model.AgentStagePlanEntry, stageDefault model.AgentStageDefault) *string {
	for _, effort := range []string{entry.Effort, plan.Effort} {
		if effort != "" {
			return &effort
		}
	}
	return stageDefault.Effort.Effort
}

func stageEntry(plan model.AgentStagePlan, stage string) model.AgentStagePlanEntry {
	for _, entry := range plan.Stages {
		if entry.Name == stage {
			return entry
		}
	}
	return model.AgentStagePlanEntry{}
}

// The gateway rejects a stage over its limit, so the tracker must never send one.
func capTurns(maxTurns int, gw *model.AgentGateway) int {
	if gw.MaxTurnsLimit != nil && maxTurns > *gw.MaxTurnsLimit {
		return *gw.MaxTurnsLimit
	}
	return maxTurns
}
