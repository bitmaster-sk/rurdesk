package agent

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func messageOfKind(id int64, kind constants.MessageKind) *model.Message {
	return &model.Message{IdMessage: id, MessageKind: kind}
}

// TestStageArtifactContext verifies a stage's own latest output is never
// "approved" while being re-executed — reaching the stage again means the
// user rejected it — while earlier stages' artifacts are genuinely approved.
func TestStageArtifactContext(t *testing.T) {
	design := messageOfKind(1, constants.MessageKindDesign)
	implPlan := messageOfKind(2, constants.MessageKindImplementationPlan)
	messages := []*model.Message{
		messageOfKind(0, constants.MessageKindComment),
		design,
		implPlan,
	}

	t.Run("design stage: previous design is rejected, nothing is approved", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageDesign, messages)
		assert.Nil(t, artifacts.ApprovedDesign)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		require.NotNil(t, artifacts.RejectedOutput)
		assert.Equal(t, design.IdMessage, artifacts.RejectedOutput.IdMessage)
	})

	t.Run("design stage first attempt: no design message yet, no rejected output", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageDesign, []*model.Message{messageOfKind(0, constants.MessageKindComment)})
		assert.Nil(t, artifacts.RejectedOutput)
		assert.Nil(t, artifacts.ApprovedDesign)
	})

	t.Run("implementation plan stage: design approved, previous plan rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageImplementationPlan, messages)
		require.NotNil(t, artifacts.ApprovedDesign)
		assert.Equal(t, design.IdMessage, artifacts.ApprovedDesign.IdMessage)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		require.NotNil(t, artifacts.RejectedOutput)
		assert.Equal(t, implPlan.IdMessage, artifacts.RejectedOutput.IdMessage)
	})

	t.Run("implementation stage: both artifacts approved, nothing rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageImplementation, messages)
		require.NotNil(t, artifacts.ApprovedDesign)
		require.NotNil(t, artifacts.ApprovedImplPlan)
		assert.Equal(t, implPlan.IdMessage, artifacts.ApprovedImplPlan.IdMessage)
		assert.Nil(t, artifacts.RejectedOutput)
	})

	t.Run("brainstorming stage: nothing approved, nothing rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageBrainstorming, messages)
		assert.Nil(t, artifacts.ApprovedDesign)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		assert.Nil(t, artifacts.RejectedOutput)
	})

	t.Run("latest message of a kind wins", func(t *testing.T) {
		designV2 := messageOfKind(3, constants.MessageKindDesign)
		artifacts := stageArtifactContext(constants.StageDesign, append(messages, designV2))
		assert.Equal(t, designV2.IdMessage, artifacts.RejectedOutput.IdMessage)
	})
}
