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

type AgentStagePlanService struct {
	pool          *pgxpool.Pool
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
}

func NewAgentStagePlanService(pool *pgxpool.Pool, agentRunRepo *repository.AgentRunRepository, agentTaskRepo *repository.AgentTaskRepository) *AgentStagePlanService {
	return &AgentStagePlanService{pool: pool, agentRunRepo: agentRunRepo, agentTaskRepo: agentTaskRepo}
}

func (s *AgentStagePlanService) BuildFromChoices(kind string, choices model.RunChoices) (json.RawMessage, error) {
	plan := newPlan(kind, choices.IdsSkillByStage, choices.IsShortWorkflow)
	applyModelChoices(&plan, nil, choices)
	return marshalPlan(plan)
}

func newPlan(kind string, idsSkillByStage map[string][]int64, isShortWorkflow bool) model.AgentStagePlan {
	stages := constants.StagesOfKind(kind)
	plan := model.AgentStagePlan{Stages: make([]model.AgentStagePlanEntry, len(stages))}
	for i, def := range stages {
		plan.Stages[i] = model.AgentStagePlanEntry{
			Name:      def.Name,
			Skippable: def.Skippable,
			Skip:      isShortWorkflow && slices.Contains(constants.ShortWorkflowSkippedStages, def.Name),
			IdsSkill:  idsSkillByStage[def.Name],
		}
	}
	return plan
}

func marshalPlan(plan model.AgentStagePlan) (json.RawMessage, error) {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("marshalling stage plan: %w", err)
	}
	return planJSON, nil
}

func (s *AgentStagePlanService) Choices(stagePlan json.RawMessage) model.RunChoices {
	plan, err := s.Parse(stagePlan)
	if err != nil {
		return model.RunChoices{}
	}
	choices := model.RunChoices{IsShortWorkflow: isShortPlan(plan), Model: plan.Model}
	for _, entry := range plan.Stages {
		if len(entry.IdsSkill) > 0 {
			if choices.IdsSkillByStage == nil {
				choices.IdsSkillByStage = map[string][]int64{}
			}
			choices.IdsSkillByStage[entry.Name] = entry.IdsSkill
		}
		if entry.Model != "" {
			if choices.ModelByStage == nil {
				choices.ModelByStage = map[string]string{}
			}
			choices.ModelByStage[entry.Name] = entry.Model
		}
		if entry.MaxTurns > 0 {
			if choices.MaxTurnsByStage == nil {
				choices.MaxTurnsByStage = map[string]int{}
			}
			choices.MaxTurnsByStage[entry.Name] = entry.MaxTurns
		}
	}
	return choices
}

func (s *AgentStagePlanService) Parse(stagePlan json.RawMessage) (model.AgentStagePlan, error) {
	var plan model.AgentStagePlan
	if err := json.Unmarshal(stagePlan, &plan); err != nil {
		return model.AgentStagePlan{}, fmt.Errorf("unmarshalling stage plan: %w", err)
	}
	return plan, nil
}

func (s *AgentStagePlanService) IsShortWorkflow(stagePlan json.RawMessage) bool {
	plan, err := s.Parse(stagePlan)
	if err != nil {
		return false
	}
	return isShortPlan(plan)
}

func isShortPlan(plan model.AgentStagePlan) bool {
	skipped := 0
	for _, entry := range plan.Stages {
		if entry.Skip && slices.Contains(constants.ShortWorkflowSkippedStages, entry.Name) {
			skipped++
		}
	}
	return skipped == len(constants.ShortWorkflowSkippedStages)
}

// An unknown stage or an unparseable plan yields none, so the stage still
// dispatches — without skills.
func (s *AgentStagePlanService) IdsSkillForStage(stagePlan json.RawMessage, stage string) []int64 {
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
func (s *AgentStagePlanService) SetStagePlanEntry(ctx context.Context, idRun int64, req model.UpdateAgentRunStagePlanEntryReq) (*model.AgentRun, error) {
	return s.rewriteLocked(ctx, idRun, func(plan *model.AgentStagePlan, _ []*model.AgentTask) error {
		for i := range plan.Stages {
			if plan.Stages[i].Name == req.Stage {
				plan.Stages[i].IdsSkill = req.IdsSkill
				plan.Stages[i].Model = req.Model
				plan.Stages[i].MaxTurns = req.MaxTurns
				return nil
			}
		}
		return errs.ErrStageNotInPlan
	})
}

// Takes the same run row lock as the scheduler, so no stage is dispatched under a stale plan.
func (s *AgentStagePlanService) SetShortWorkflow(ctx context.Context, idRun int64, isShortWorkflow bool) (*model.AgentRun, error) {
	return s.rewriteLocked(ctx, idRun, func(plan *model.AgentStagePlan, tasks []*model.AgentTask) error {
		if isWorkflowLocked(tasks) {
			return errs.ErrWorkflowLocked
		}
		applyShortWorkflow(plan, isShortWorkflow)
		return nil
	})
}

func (s *AgentStagePlanService) ApplyChoices(ctx context.Context, idRun int64, choices model.RunChoices) (*model.AgentRun, error) {
	return s.rewriteLocked(ctx, idRun, func(plan *model.AgentStagePlan, tasks []*model.AgentTask) error {
		return applyChoices(plan, tasks, choices)
	})
}

func (s *AgentStagePlanService) rewriteLocked(ctx context.Context, idRun int64, rewrite func(*model.AgentStagePlan, []*model.AgentTask) error) (*model.AgentRun, error) {
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

		planJSON, err := marshalPlan(plan)
		if err != nil {
			return err
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

func applyChoices(plan *model.AgentStagePlan, tasks []*model.AgentTask, choices model.RunChoices) error {
	if choices.IsShortWorkflow != isShortPlan(*plan) {
		if isWorkflowLocked(tasks) {
			return errs.ErrWorkflowLocked
		}
		applyShortWorkflow(plan, choices.IsShortWorkflow)
	}
	dispatched := DispatchedStages(tasks)
	for i := range plan.Stages {
		name := plan.Stages[i].Name
		if constants.IsSkillStage(name) && !dispatched[name] {
			plan.Stages[i].IdsSkill = choices.IdsSkillByStage[name]
		}
	}
	applyModelChoices(plan, dispatched, choices)
	return nil
}

func applyModelChoices(plan *model.AgentStagePlan, dispatched map[string]bool, choices model.RunChoices) {
	plan.Model = choices.Model
	for i := range plan.Stages {
		name := plan.Stages[i].Name
		if constants.IsSkillStage(name) && !dispatched[name] {
			plan.Stages[i].Model = choices.ModelByStage[name]
			plan.Stages[i].MaxTurns = choices.MaxTurnsByStage[name]
		}
	}
}

func isWorkflowLocked(tasks []*model.AgentTask) bool {
	for _, task := range tasks {
		if task.Status != constants.TaskStatusCancelled && slices.Contains(constants.ShortWorkflowLockingStages, task.Stage) {
			return true
		}
	}
	return false
}

func applyShortWorkflow(plan *model.AgentStagePlan, isShortWorkflow bool) {
	for i := range plan.Stages {
		if slices.Contains(constants.ShortWorkflowSkippedStages, plan.Stages[i].Name) {
			plan.Stages[i].Skip = isShortWorkflow
		}
	}
}
