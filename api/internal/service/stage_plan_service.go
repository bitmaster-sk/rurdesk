package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StagePlanService struct {
	pool         *pgxpool.Pool
	agentRunRepo *repository.AgentRunRepository
}

func NewStagePlanService(pool *pgxpool.Pool, agentRunRepo *repository.AgentRunRepository) *StagePlanService {
	return &StagePlanService{pool: pool, agentRunRepo: agentRunRepo}
}

func (s *StagePlanService) Build(idsSkillByStage map[string][]int64) (json.RawMessage, error) {
	plan := model.StagePlan{Stages: make([]model.StagePlanEntry, len(constants.StageDefinitions))}
	for i, def := range constants.StageDefinitions {
		plan.Stages[i] = model.StagePlanEntry{
			Name:      def.Name,
			Skippable: def.Skippable,
			Skip:      false,
			IdsSkill:  idsSkillByStage[def.Name],
		}
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("marshalling stage plan: %w", err)
	}
	return planJSON, nil
}

func (s *StagePlanService) Parse(stagePlan json.RawMessage) (model.StagePlan, error) {
	var plan model.StagePlan
	if err := json.Unmarshal(stagePlan, &plan); err != nil {
		return model.StagePlan{}, fmt.Errorf("unmarshalling stage plan: %w", err)
	}
	return plan, nil
}

// The inverse of Build. An unparseable plan yields no skills rather than an
// error, so a run stays creatable.
func (s *StagePlanService) IdsSkillByStage(stagePlan json.RawMessage) map[string][]int64 {
	plan, err := s.Parse(stagePlan)
	if err != nil {
		return nil
	}
	byStage := make(map[string][]int64, len(plan.Stages))
	for _, entry := range plan.Stages {
		if len(entry.IdsSkill) > 0 {
			byStage[entry.Name] = entry.IdsSkill
		}
	}
	return byStage
}

// An unknown stage or an unparseable plan yields none, so the stage still
// dispatches — without skills.
func (s *StagePlanService) IdsSkillForStage(stagePlan json.RawMessage, stage string) []int64 {
	plan, err := s.Parse(stagePlan)
	if err != nil {
		return nil
	}
	for _, entry := range plan.Stages {
		if entry.Name == stage {
			return entry.IdsSkill
		}
	}
	return nil
}

// Runs on a locked row: two stages of one run changed at once would otherwise
// lose a write. The already-dispatched guard lives in the controller.
func (s *StagePlanService) SetStageSkills(ctx context.Context, idRun int64, stage string, idsSkill []int64) (*model.AgentRun, error) {
	var updated *model.AgentRun
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		run, err := s.agentRunRepo.LoadByIdForUpdate(ctx, idRun)
		if err != nil {
			return err
		}
		plan, err := s.Parse(run.StagePlan)
		if err != nil {
			return err
		}

		found := false
		for i := range plan.Stages {
			if plan.Stages[i].Name == stage {
				plan.Stages[i].IdsSkill = idsSkill
				found = true
			}
		}
		if !found {
			return errs.ErrStageNotInPlan
		}

		planJSON, err := json.Marshal(plan)
		if err != nil {
			return fmt.Errorf("marshalling stage plan: %w", err)
		}
		updated, err = s.agentRunRepo.UpdateStagePlan(ctx, idRun, planJSON)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
