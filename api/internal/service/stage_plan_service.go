package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StagePlanService struct {
	pool          *pgxpool.Pool
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
}

func NewStagePlanService(pool *pgxpool.Pool, agentRunRepo *repository.AgentRunRepository, agentTaskRepo *repository.AgentTaskRepository) *StagePlanService {
	return &StagePlanService{pool: pool, agentRunRepo: agentRunRepo, agentTaskRepo: agentTaskRepo}
}

func (s *StagePlanService) Build(idsSkillByStage map[string][]int64, isShortWorkflow bool) (json.RawMessage, error) {
	plan := model.StagePlan{Stages: make([]model.StagePlanEntry, len(constants.StageDefinitions))}
	for i, def := range constants.StageDefinitions {
		plan.Stages[i] = model.StagePlanEntry{
			Name:      def.Name,
			Skippable: def.Skippable,
			Skip:      isShortWorkflow && slices.Contains(constants.ShortWorkflowSkippedStages, def.Name),
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

func (s *StagePlanService) IsShortWorkflow(stagePlan json.RawMessage) bool {
	plan, err := s.Parse(stagePlan)
	if err != nil {
		return false
	}
	return isShortPlan(plan)
}

func isShortPlan(plan model.StagePlan) bool {
	skipped := 0
	for _, entry := range plan.Stages {
		if entry.Skip && slices.Contains(constants.ShortWorkflowSkippedStages, entry.Name) {
			skipped++
		}
	}
	return skipped == len(constants.ShortWorkflowSkippedStages)
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
	return s.rewriteLocked(ctx, idRun, func(plan *model.StagePlan, _ []*model.AgentTask) error {
		for i := range plan.Stages {
			if plan.Stages[i].Name == stage {
				plan.Stages[i].IdsSkill = idsSkill
				return nil
			}
		}
		return errs.ErrStageNotInPlan
	})
}

// Takes the same run row lock as the scheduler, so no stage is dispatched under a stale plan.
func (s *StagePlanService) SetShortWorkflow(ctx context.Context, idRun int64, isShortWorkflow bool) (*model.AgentRun, error) {
	return s.rewriteLocked(ctx, idRun, func(plan *model.StagePlan, tasks []*model.AgentTask) error {
		if isWorkflowLocked(tasks) {
			return errs.ErrWorkflowLocked
		}
		applyShortWorkflow(plan, isShortWorkflow)
		return nil
	})
}

func (s *StagePlanService) ApplyChoices(ctx context.Context, idRun int64, idsSkillByStage map[string][]int64, isShortWorkflow bool) (*model.AgentRun, error) {
	return s.rewriteLocked(ctx, idRun, func(plan *model.StagePlan, tasks []*model.AgentTask) error {
		return applyChoices(plan, tasks, idsSkillByStage, isShortWorkflow)
	})
}

func (s *StagePlanService) rewriteLocked(ctx context.Context, idRun int64, rewrite func(*model.StagePlan, []*model.AgentTask) error) (*model.AgentRun, error) {
	var updated *model.AgentRun
	err := extctx.JoinTx(ctx, s.pool, func(ctx context.Context) error {
		run, err := s.agentRunRepo.LoadByIdForUpdate(ctx, idRun)
		if err != nil {
			return err
		}
		tasks, err := s.agentTaskRepo.LoadByRun(ctx, idRun)
		if err != nil {
			return err
		}
		plan, err := s.Parse(run.StagePlan)
		if err != nil {
			return err
		}
		if err := rewrite(&plan, tasks); err != nil {
			return err
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

// A cancelled stage is dispatched again with a prompt built from the current plan.
func DispatchedStages(tasks []*model.AgentTask) map[string]bool {
	dispatched := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if task.Status != constants.TaskStatusCancelled {
			dispatched[task.Stage] = true
		}
	}
	return dispatched
}

func applyChoices(plan *model.StagePlan, tasks []*model.AgentTask, idsSkillByStage map[string][]int64, isShortWorkflow bool) error {
	if isShortWorkflow != isShortPlan(*plan) {
		if isWorkflowLocked(tasks) {
			return errs.ErrWorkflowLocked
		}
		applyShortWorkflow(plan, isShortWorkflow)
	}
	dispatched := DispatchedStages(tasks)
	for i := range plan.Stages {
		name := plan.Stages[i].Name
		if constants.IsSkillStage(name) && !dispatched[name] {
			plan.Stages[i].IdsSkill = idsSkillByStage[name]
		}
	}
	return nil
}

func isWorkflowLocked(tasks []*model.AgentTask) bool {
	for _, task := range tasks {
		if task.Status != constants.TaskStatusCancelled && slices.Contains(constants.ShortWorkflowLockingStages, task.Stage) {
			return true
		}
	}
	return false
}

func applyShortWorkflow(plan *model.StagePlan, isShortWorkflow bool) {
	for i := range plan.Stages {
		if slices.Contains(constants.ShortWorkflowSkippedStages, plan.Stages[i].Name) {
			plan.Stages[i].Skip = isShortWorkflow
		}
	}
}
