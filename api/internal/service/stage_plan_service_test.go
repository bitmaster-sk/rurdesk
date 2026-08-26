package service

import (
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagePlanBuildCoversEveryStageInOrder(t *testing.T) {
	planner := NewStagePlanService(nil, nil)

	planJSON, err := planner.Build(map[string][]int64{"design": {7, 9}})
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
	planner := NewStagePlanService(nil, nil)

	planJSON, err := planner.Build(map[string][]int64{"nope": {1}})
	require.NoError(t, err)

	plan, err := planner.Parse(planJSON)
	require.NoError(t, err)
	for _, entry := range plan.Stages {
		assert.Nil(t, entry.IdsSkill)
	}
}

func TestIdsSkillForStage(t *testing.T) {
	planner := NewStagePlanService(nil, nil)

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
	planner := NewStagePlanService(nil, nil)

	original := map[string][]int64{"design": {7, 9}, "implementation": {3}}

	planJSON, err := planner.Build(original)
	require.NoError(t, err)

	assert.Equal(t, original, planner.IdsSkillByStage(planJSON), "a restart carries the same choices over")
	assert.Nil(t, planner.IdsSkillByStage(json.RawMessage(`not json`)))
}
