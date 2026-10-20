package service

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type AgentTaskConfig struct {
	Model        *string
	MaxTurns     int
	FallbackFrom *string
}

type AgentTaskConfigService struct {
	settings    *AppSettingsService
	agentGwRepo *repository.AgentGatewayRepository
}

func NewAgentTaskConfigService(settings *AppSettingsService, agentGwRepo *repository.AgentGatewayRepository) *AgentTaskConfigService {
	return &AgentTaskConfigService{settings: settings, agentGwRepo: agentGwRepo}
}

func (s *AgentTaskConfigService) GetDefaultMaxTurns(gw *model.AgentGateway) (plan, implement int) {
	plan, implement = s.settings.AgentMaxTurnsPlan(), s.settings.AgentMaxTurnsImplement()
	if gw.MaxTurnsPlan != nil {
		plan = *gw.MaxTurnsPlan
	}
	if gw.MaxTurnsImplement != nil {
		implement = *gw.MaxTurnsImplement
	}
	return capTurns(plan, gw), capTurns(implement, gw)
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
		entry.MaxTurnsPlan, entry.MaxTurnsImplement = s.GetDefaultMaxTurns(gateway)
		entry.MaxTurnsLimit = gateway.MaxTurnsLimit
	}
	return nil
}

func (s *AgentTaskConfigService) ValidateChoices(gateway *model.AgentGateway, choices model.RunChoices) error {
	stages := slices.Collect(maps.Keys(choices.IdsSkillByStage))
	stages = slices.AppendSeq(stages, maps.Keys(choices.ModelByStage))
	stages = slices.AppendSeq(stages, maps.Keys(choices.MaxTurnsByStage))
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
	models := append(slices.Collect(maps.Values(choices.ModelByStage)), choices.Model)
	for _, chosen := range models {
		if chosen != "" && !gateway.IsSupportedModel(chosen) {
			return errs.ErrAgentModelNotOffered.WithMessage("the agent does not offer the model " + chosen)
		}
	}
	return nil
}

func (s *AgentTaskConfigService) BuildAgentTaskConfig(plan model.AgentStagePlan, stage string, gw *model.AgentGateway) AgentTaskConfig {
	entry := stageEntry(plan, stage)
	taskConfig := AgentTaskConfig{MaxTurns: capTurns(entry.MaxTurns, gw)}
	if taskConfig.MaxTurns == 0 {
		planTurns, implementTurns := s.GetDefaultMaxTurns(gw)
		taskConfig.MaxTurns = planTurns
		if stage == constants.StageImplementation {
			taskConfig.MaxTurns = implementTurns
		}
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
	return taskConfig
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
