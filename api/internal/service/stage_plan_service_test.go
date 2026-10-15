package service

import (
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagePlanBuildCoversEveryStageInOrder(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	planJSON, err := planner.Build(map[string][]int64{"design": {7, 9}}, false)
	require.NoError(t, err)

	plan, err := planner.Parse(planJSON)
	require.NoError(t, err)
	require.Len(t, plan.Stages, len(constants.StageDefinitions))

	for i, def := range constants.StageDefinitions {
		assert.Equal(t, def.Name, plan.Stages[i].Name, "canonical stage order is preserved")
		assert.Equal(t, def.Skippable, plan.Stages[i].Skippable)
		assert.False(t, plan.Stages[i].Skip, "a new run skips nothing")
	}
	assert.Equal(t, []int64{7, 9}, planner.IdsSkillForStage(planJSON, "design"))
	assert.Nil(t, planner.IdsSkillForStage(planJSON, "implementation"), "a stage with no chosen skills carries none")
}

func TestStagePlanBuildIgnoresUnknownStage(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	planJSON, err := planner.Build(map[string][]int64{"nope": {1}}, false)
	require.NoError(t, err)

	plan, err := planner.Parse(planJSON)
	require.NoError(t, err)
	for _, entry := range plan.Stages {
		assert.Nil(t, entry.IdsSkill)
	}
}

func TestIdsSkillForStage(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	plan := json.RawMessage(`{"stages":[
		{"name":"pickup","skippable":false,"skip":false},
		{"name":"design","skippable":true,"skip":false,"idsSkill":[7,9]},
		{"name":"implementation","skippable":false,"skip":false}
	]}`)

	tests := []struct {
		name     string
		plan     json.RawMessage
		stage    string
		expected []int64
	}{
		{"stage with skills", plan, "design", []int64{7, 9}},
		{"stage without skills", plan, "implementation", nil},
		{"unknown stage", plan, "nope", nil},
		{"malformed plan", json.RawMessage(`not json`), "design", nil},
		{"empty plan", nil, "design", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, planner.IdsSkillForStage(tc.plan, tc.stage))
		})
	}
}

func TestIdsSkillByStageRoundTripsThroughBuild(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	original := map[string][]int64{"design": {7, 9}, "implementation": {3}}

	planJSON, err := planner.Build(original, false)
	require.NoError(t, err)

	assert.Equal(t, original, planner.IdsSkillByStage(planJSON), "a restart carries the same choices over")
	assert.Nil(t, planner.IdsSkillByStage(json.RawMessage(`not json`)))
}

func TestStagePlanBuildShortSkipsDesignAndPlanOnly(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	planJSON, err := planner.Build(nil, true)
	require.NoError(t, err)

	plan, err := planner.Parse(planJSON)
	require.NoError(t, err)
	skipped := map[string]bool{}
	for _, entry := range plan.Stages {
		skipped[entry.Name] = entry.Skip
	}
	assert.Equal(t, map[string]bool{
		constants.StagePickup:             false,
		constants.StageBrainstorming:      false,
		constants.StageDesign:             true,
		constants.StageImplementationPlan: true,
		constants.StageImplementation:     false,
	}, skipped)
}

func TestIsShortWorkflow(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)

	short, err := planner.Build(nil, true)
	require.NoError(t, err)
	full, err := planner.Build(nil, false)
	require.NoError(t, err)

	assert.True(t, planner.IsShortWorkflow(short))
	assert.False(t, planner.IsShortWorkflow(full))
	assert.False(t, planner.IsShortWorkflow(json.RawMessage(`{"stages":[{"name":"design","skip":true}]}`)), "skipping only the design is not the short workflow")
	assert.False(t, planner.IsShortWorkflow(json.RawMessage(`not json`)))
}

func stageTask(stage, status string) *model.AgentTask {
	return &model.AgentTask{Stage: stage, Status: status}
}

func TestDispatchedStagesIgnoresCancelledTasks(t *testing.T) {
	dispatched := DispatchedStages([]*model.AgentTask{
		stageTask(constants.StageBrainstorming, constants.TaskStatusCompleted),
		stageTask(constants.StageDesign, constants.TaskStatusCancelled),
		stageTask(constants.StageImplementationPlan, constants.TaskStatusFailed),
	})

	assert.Equal(t, map[string]bool{
		constants.StageBrainstorming:      true,
		constants.StageImplementationPlan: true,
	}, dispatched)
}

func TestApplyChoices(t *testing.T) {
	planner := NewStagePlanService(nil, nil, nil)
	brainstormingDone := stageTask(constants.StageBrainstorming, constants.TaskStatusCompleted)
	designCancelled := stageTask(constants.StageDesign, constants.TaskStatusCancelled)
	designDone := stageTask(constants.StageDesign, constants.TaskStatusCompleted)

	tests := []struct {
		name            string
		isShortCurrent  bool
		tasks           []*model.AgentTask
		isShortWorkflow bool
		expectedSkills  map[string][]int64
		expectedShort   bool
		expectedErr     error
	}{
		{
			name:            "replaces the skills of stages not yet dispatched",
			tasks:           []*model.AgentTask{brainstormingDone},
			isShortWorkflow: false,
			expectedSkills:  map[string][]int64{constants.StageBrainstorming: {1}, constants.StageDesign: {4}, constants.StageImplementation: {5}},
		},
		{
			name:            "a cancelled stage runs again, so it takes the new skills",
			tasks:           []*model.AgentTask{brainstormingDone, designCancelled},
			isShortWorkflow: false,
			expectedSkills:  map[string][]int64{constants.StageBrainstorming: {1}, constants.StageDesign: {4}, constants.StageImplementation: {5}},
		},
		{
			name:            "shortens the workflow while design has not started",
			tasks:           []*model.AgentTask{brainstormingDone, designCancelled},
			isShortWorkflow: true,
			expectedSkills:  map[string][]int64{constants.StageBrainstorming: {1}, constants.StageDesign: {4}, constants.StageImplementation: {5}},
			expectedShort:   true,
		},
		{
			name:            "refuses to shorten the workflow once design ran",
			tasks:           []*model.AgentTask{brainstormingDone, designDone},
			isShortWorkflow: true,
			expectedErr:     errs.ErrWorkflowLocked,
		},
		{
			name:            "an unchanged workflow is no change, even when locked",
			isShortCurrent:  true,
			tasks:           []*model.AgentTask{brainstormingDone, designDone},
			isShortWorkflow: true,
			expectedSkills:  map[string][]int64{constants.StageBrainstorming: {1}, constants.StageImplementation: {5}},
			expectedShort:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			planJSON, err := planner.Build(map[string][]int64{constants.StageBrainstorming: {1}}, tc.isShortCurrent)
			require.NoError(t, err)
			plan, err := planner.Parse(planJSON)
			require.NoError(t, err)

			err = applyChoices(&plan, tc.tasks, map[string][]int64{
				constants.StageBrainstorming:  {2},
				constants.StageDesign:         {4},
				constants.StageImplementation: {5},
			}, tc.isShortWorkflow)

			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			skills := map[string][]int64{}
			for _, entry := range plan.Stages {
				if len(entry.IdsSkill) > 0 {
					skills[entry.Name] = entry.IdsSkill
				}
			}
			assert.Equal(t, tc.expectedSkills, skills)
			assert.Equal(t, tc.expectedShort, isShortPlan(plan))
		})
	}
}
