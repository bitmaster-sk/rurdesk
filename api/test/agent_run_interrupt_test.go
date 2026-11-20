package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// An issue run whose stage the agent will not finish fails, so the user sees
// Continue/Restart: after a gateway restart, a stale heartbeat or a dispatch the
// gateway never received.

// activeRun returns an issue run in progress with an active implementation task.
func (fx *runGitRepoFixture) activeRun(t *testing.T, name string) (*model.AgentRun, *model.AgentTask) {
	t.Helper()
	ctx := context.Background()
	idProject := fx.project(t, name)
	fx.integration(t, idProject, "null")
	run := fx.run(t, idProject)
	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, fx.idAgent, constants.StageImplementation, 1)
	require.NoError(t, err)
	task, err = injector.GetAgentTaskRepository().TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive)
	require.NoError(t, err)
	run, err = injector.GetAgentRunRepository().TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseInProgress,
		constants.ActorTypeSystem, nil, "test")
	require.NoError(t, err)
	return run, task
}

func (fx *runGitRepoFixture) reloadTask(t *testing.T, idTask int64) *model.AgentTask {
	t.Helper()
	task, err := injector.GetAgentTaskRepository().LoadById(context.Background(), idTask)
	require.NoError(t, err)
	return task
}

func requireFailedTask(t *testing.T, task *model.AgentTask, reason string) {
	t.Helper()
	require.Equal(t, constants.TaskStatusFailed, task.Status)
	require.NotNil(t, task.ErrorReason)
	require.Equal(t, reason, *task.ErrorReason)
}

func TestGatewayRestartFailsAnIssueRun(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	run, task := fx.activeRun(t, "interrupt-gateway-restart")

	res := Request(t, fx.app, "POST", "/api/private/agent/gateway/recovered", `{}`, fx.agentKey)
	require.Equal(t, http.StatusOK, res.StatusCode)

	failed := fx.reloadRun(t, run.IdRun)
	require.Equal(t, constants.PhaseFailed, failed.Phase)
	require.NotNil(t, failed.ErrorMessage)
	require.Equal(t, constants.FailReasonGatewayRestart, *failed.ErrorMessage)
	requireFailedTask(t, fx.reloadTask(t, task.IdTask), constants.FailReasonGatewayRestart)
}

func TestHeartbeatSweepFailsAnIssueRun(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	run, task := fx.activeRun(t, "interrupt-heartbeat")
	_, err := fx.app.Pool.Exec(context.Background(),
		`UPDATE agent.task SET last_heartbeat_at = now() - interval '11 minutes' WHERE id_task = $1`, task.IdTask)
	require.NoError(t, err)

	injector.GetSweep().HeartbeatSweepOnce(context.Background())

	failed := fx.reloadRun(t, run.IdRun)
	require.Equal(t, constants.PhaseFailed, failed.Phase)
	require.NotNil(t, failed.ErrorMessage)
	require.Equal(t, constants.FailReasonHeartbeatStale, *failed.ErrorMessage)
	requireFailedTask(t, fx.reloadTask(t, task.IdTask), constants.FailReasonHeartbeatStale)
}

func TestUndispatchableStageFailsAnIssueRun(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	ctx := context.Background()
	idProject := fx.project(t, "interrupt-dispatch")
	fx.integration(t, idProject, "null")
	run := fx.run(t, idProject)
	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, fx.idAgent, constants.StagePickup, 1)
	require.NoError(t, err)
	task, err = injector.GetAgentTaskRepository().TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive)
	require.NoError(t, err)
	_, err = fx.app.Pool.Exec(ctx, `DELETE FROM agent.gateway WHERE id_user_agent = $1`, fx.idAgent)
	require.NoError(t, err)

	injector.GetDispatcher().DispatchStage(ctx, &agent.Dispatch{Run: run, Task: task})

	require.Eventually(t, func() bool {
		return fx.reloadRun(t, run.IdRun).Phase == constants.PhaseFailed
	}, 10*time.Second, 50*time.Millisecond, "a stage without a gateway to send it to fails the run")
	failedTask := fx.reloadTask(t, task.IdTask)
	requireFailedTask(t, failedTask, "dispatch_failed")
	require.NotNil(t, failedTask.ErrorDetail)
	require.Contains(t, *failedTask.ErrorDetail, "no gateway configured")
	require.Nil(t, fx.reloadRun(t, run.IdRun).ErrorMessage, "the reason stays on the task")
}
