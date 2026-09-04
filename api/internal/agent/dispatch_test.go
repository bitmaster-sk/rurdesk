package agent

import (
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var baseTime = time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

func messageOfKind(id int64, kind constants.MessageKind) *model.Message {
	return &model.Message{IdMessage: id, MessageKind: kind}
}

func messageAt(id int64, kind constants.MessageKind, offset time.Duration) *model.Message {
	return &model.Message{IdMessage: id, MessageKind: kind, CreatedAt: baseTime.Add(offset)}
}

func idsOf(messages []*model.Message) []int64 {
	out := make([]int64, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.IdMessage)
	}
	return out
}

func completedStageTaskWithOutput(stage string, attemptNo int, idResultMessage int64, createdOffset time.Duration) *model.AgentTask {
	id := idResultMessage
	return &model.AgentTask{
		IdTask:          idResultMessage,
		Stage:           stage,
		AttemptNo:       attemptNo,
		Status:          constants.TaskStatusCompleted,
		IdResultMessage: &id,
		CreatedAt:       baseTime.Add(createdOffset),
	}
}

// descendingFeed mirrors LoadIssueMessages, which returns newest first.
func descendingFeed(messages ...*model.Message) []*model.Message {
	out := make([]*model.Message, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		out = append(out, messages[i])
	}
	return out
}

// TestStageArtifactContext verifies a stage's own latest output is never
// "approved" while being re-executed — reaching the stage again means the
// user rejected it — while earlier stages' artifacts are genuinely approved.
func TestStageArtifactContext(t *testing.T) {
	comment := messageOfKind(10, constants.MessageKindComment)
	design := messageOfKind(11, constants.MessageKindDesign)
	implPlan := messageOfKind(12, constants.MessageKindImplementationPlan)
	messages := descendingFeed(comment, design, implPlan)
	tasks := []*model.AgentTask{
		completedStageTaskWithOutput(constants.StageDesign, 1, design.IdMessage, time.Minute),
		completedStageTaskWithOutput(constants.StageImplementationPlan, 1, implPlan.IdMessage, 2*time.Minute),
	}

	t.Run("design stage: previous design is rejected, nothing is approved", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageDesign, tasks, messages)
		assert.Nil(t, artifacts.ApprovedDesign)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		require.NotNil(t, artifacts.RejectedOutput)
		assert.Equal(t, design.IdMessage, artifacts.RejectedOutput.IdMessage)
	})

	t.Run("design stage first attempt: no design message yet, no rejected output", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageDesign, nil, descendingFeed(comment))
		assert.Nil(t, artifacts.RejectedOutput)
		assert.Nil(t, artifacts.ApprovedDesign)
	})

	t.Run("implementation plan stage: design approved, previous plan rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageImplementationPlan, tasks, messages)
		require.NotNil(t, artifacts.ApprovedDesign)
		assert.Equal(t, design.IdMessage, artifacts.ApprovedDesign.IdMessage)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		require.NotNil(t, artifacts.RejectedOutput)
		assert.Equal(t, implPlan.IdMessage, artifacts.RejectedOutput.IdMessage)
	})

	t.Run("implementation stage: both artifacts approved, nothing rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageImplementation, tasks, messages)
		require.NotNil(t, artifacts.ApprovedDesign)
		require.NotNil(t, artifacts.ApprovedImplPlan)
		assert.Equal(t, implPlan.IdMessage, artifacts.ApprovedImplPlan.IdMessage)
		assert.Nil(t, artifacts.RejectedOutput)
	})

	t.Run("brainstorming stage: nothing approved, nothing rejected", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageBrainstorming, tasks, messages)
		assert.Nil(t, artifacts.ApprovedDesign)
		assert.Nil(t, artifacts.ApprovedImplPlan)
		assert.Nil(t, artifacts.RejectedOutput)
	})
}

// After a revision, later stages must see the newest attempt's output, not the
// first one the user already rejected.
func TestStageArtifactContextPicksNewestAttempt(t *testing.T) {
	designV1 := messageOfKind(11, constants.MessageKindDesign)
	designV2 := messageOfKind(13, constants.MessageKindDesign)
	planV1 := messageOfKind(12, constants.MessageKindImplementationPlan)
	planV2 := messageOfKind(14, constants.MessageKindImplementationPlan)
	messages := descendingFeed(designV1, planV1, designV2, planV2)
	tasks := []*model.AgentTask{
		completedStageTaskWithOutput(constants.StageDesign, 1, designV1.IdMessage, time.Minute),
		completedStageTaskWithOutput(constants.StageImplementationPlan, 1, planV1.IdMessage, 2*time.Minute),
		completedStageTaskWithOutput(constants.StageDesign, 2, designV2.IdMessage, 3*time.Minute),
		completedStageTaskWithOutput(constants.StageImplementationPlan, 2, planV2.IdMessage, 4*time.Minute),
	}

	artifacts := stageArtifactContext(constants.StageImplementation, tasks, messages)
	require.NotNil(t, artifacts.ApprovedDesign)
	assert.Equal(t, designV2.IdMessage, artifacts.ApprovedDesign.IdMessage)
	require.NotNil(t, artifacts.ApprovedImplPlan)
	assert.Equal(t, planV2.IdMessage, artifacts.ApprovedImplPlan.IdMessage)

	t.Run("selection does not depend on input order", func(t *testing.T) {
		reversedTasks := []*model.AgentTask{tasks[3], tasks[2], tasks[1], tasks[0]}
		reordered := stageArtifactContext(constants.StageImplementation, reversedTasks, descendingFeed(messages...))
		require.NotNil(t, reordered.ApprovedDesign)
		assert.Equal(t, designV2.IdMessage, reordered.ApprovedDesign.IdMessage)
		require.NotNil(t, reordered.ApprovedImplPlan)
		assert.Equal(t, planV2.IdMessage, reordered.ApprovedImplPlan.IdMessage)
	})
}

// Restart starts a new run but leaves the old run's design and plan on the
// issue; the new run must not treat them as its own output.
func TestStageArtifactContextIgnoresForeignMessages(t *testing.T) {
	previousRunDesign := messageOfKind(11, constants.MessageKindDesign)
	previousRunPlan := messageOfKind(12, constants.MessageKindImplementationPlan)
	messages := descendingFeed(previousRunDesign, previousRunPlan)

	t.Run("first design attempt of a restarted run has no rejected output", func(t *testing.T) {
		artifacts := stageArtifactContext(constants.StageDesign, nil, messages)
		assert.Nil(t, artifacts.RejectedOutput)
	})

	t.Run("implementation plan stage sees only this run's design", func(t *testing.T) {
		ownDesign := messageOfKind(21, constants.MessageKindDesign)
		tasks := []*model.AgentTask{completedStageTaskWithOutput(constants.StageDesign, 1, ownDesign.IdMessage, time.Minute)}
		artifacts := stageArtifactContext(
			constants.StageImplementationPlan, tasks, descendingFeed(previousRunDesign, previousRunPlan, ownDesign),
		)
		require.NotNil(t, artifacts.ApprovedDesign)
		assert.Equal(t, ownDesign.IdMessage, artifacts.ApprovedDesign.IdMessage)
		assert.Nil(t, artifacts.RejectedOutput)
	})
}

// Failed and in-flight attempts contribute no artifact.
func TestStageArtifactContextIgnoresUnfinishedAttempts(t *testing.T) {
	design := messageOfKind(11, constants.MessageKindDesign)
	orphan := messageOfKind(12, constants.MessageKindDesign)
	failed := completedStageTaskWithOutput(constants.StageDesign, 2, orphan.IdMessage, 2*time.Minute)
	failed.Status = constants.TaskStatusFailed
	active := &model.AgentTask{
		IdTask:    99,
		Stage:     constants.StageDesign,
		AttemptNo: 3,
		Status:    constants.TaskStatusActive,
		CreatedAt: baseTime.Add(3 * time.Minute),
	}
	tasks := []*model.AgentTask{
		completedStageTaskWithOutput(constants.StageDesign, 1, design.IdMessage, time.Minute),
		failed,
		active,
	}

	artifacts := stageArtifactContext(constants.StageDesign, tasks, descendingFeed(design, orphan))
	require.NotNil(t, artifacts.RejectedOutput)
	assert.Equal(t, design.IdMessage, artifacts.RejectedOutput.IdMessage)
}

func TestReviewThreadKeepsEveryMessageInOrder(t *testing.T) {
	firstComment := messageAt(1, constants.MessageKindComment, time.Minute)
	design := messageAt(2, constants.MessageKindDesign, 2*time.Minute)
	pushed := messageAt(3, constants.MessageKindPullRequestPushed, 3*time.Minute)
	secondComment := messageAt(4, constants.MessageKindComment, 4*time.Minute)

	thread := reviewThread(descendingFeed(firstComment, design, pushed, secondComment))

	assert.Equal(t, []int64{1, 2, 3, 4}, idsOf(thread), "oldest first, nothing dropped")
}

func TestReviewThreadKeepsCommentsOlderThanTheLastAttempt(t *testing.T) {
	instruction := messageAt(1, constants.MessageKindComment, time.Minute)
	pushed := messageAt(2, constants.MessageKindPullRequestPushed, 2*time.Minute)
	followUp := messageAt(3, constants.MessageKindComment, 3*time.Minute)

	thread := reviewThread(descendingFeed(instruction, pushed, followUp))

	assert.Contains(t, idsOf(thread), instruction.IdMessage)
	assert.Contains(t, idsOf(thread), followUp.IdMessage)
}

func TestReviewThreadOrdersEqualTimestampsById(t *testing.T) {
	first := messageAt(1, constants.MessageKindComment, time.Minute)
	second := messageAt(2, constants.MessageKindComment, time.Minute)

	assert.Equal(t, []int64{1, 2}, idsOf(reviewThread([]*model.Message{first, second})))
	assert.Equal(t, []int64{1, 2}, idsOf(reviewThread([]*model.Message{second, first})))
}

func TestReviewThreadDropsArtifactBodiesWithoutTouchingTheOriginals(t *testing.T) {
	design := messageAt(1, constants.MessageKindDesign, time.Minute)
	design.Message = "DESIGN-MD"
	plan := messageAt(2, constants.MessageKindImplementationPlan, 2*time.Minute)
	plan.Message = "PLAN-MD"
	comment := messageAt(3, constants.MessageKindComment, 3*time.Minute)
	comment.Message = "use uuid"
	feed := descendingFeed(design, plan, comment)

	thread := reviewThread(feed)

	assert.Empty(t, thread[0].Message, "design body is shipped as approvedDesign instead")
	assert.Empty(t, thread[1].Message, "plan body is shipped as approvedImplPlan instead")
	assert.Equal(t, "use uuid", thread[2].Message, "comments keep their body")

	artifacts := stageArtifactContext(constants.StageImplementation, []*model.AgentTask{
		completedStageTaskWithOutput(constants.StageDesign, 1, design.IdMessage, time.Minute),
		completedStageTaskWithOutput(constants.StageImplementationPlan, 1, plan.IdMessage, 2*time.Minute),
	}, feed)
	require.NotNil(t, artifacts.ApprovedDesign)
	assert.Equal(t, "DESIGN-MD", artifacts.ApprovedDesign.Message)
	require.NotNil(t, artifacts.ApprovedImplPlan)
	assert.Equal(t, "PLAN-MD", artifacts.ApprovedImplPlan.Message)
}

func TestReviewThreadOnEmptyFeed(t *testing.T) {
	assert.Empty(t, reviewThread(nil))
}
