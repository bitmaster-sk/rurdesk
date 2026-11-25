package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// runKillingAgent stops the agent by run, like the real adapters do: Cancel ends whichever attempt
// of the run is running at that moment.
type runKillingAgent struct {
	entered chan int64
	release chan struct{}

	mu      sync.Mutex
	running map[RunID]context.CancelFunc
}

func newRunKillingAgent() *runKillingAgent {
	return &runKillingAgent{entered: make(chan int64, 4), release: make(chan struct{}), running: map[RunID]context.CancelFunc{}}
}

func (a *runKillingAgent) Run(ctx context.Context, task Task) (RunStats, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runID := RunID(fmt.Sprintf("%d", task.IdRun))
	a.mu.Lock()
	a.running[runID] = cancel
	a.mu.Unlock()
	a.entered <- task.IdTask
	select {
	case <-a.release:
		return RunStats{}, nil
	case <-runCtx.Done():
		return RunStats{}, errors.New("agent cancelled")
	}
}

func (a *runKillingAgent) Cancel(_ context.Context, runID RunID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cancel := a.running[runID]; cancel != nil {
		cancel()
	}
	return nil
}

func cancelledEvent(idRun int64, payload string) WebhookEvent {
	return WebhookEvent{Type: "cancelled", IdRun: idRun, Payload: json.RawMessage(payload)}
}

// startStage runs the stage in the background; the returned channel closes when it ends.
func startStage(orchestrator *Orchestrator, task Task) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		orchestrator.runStage(task)
		close(done)
	}()
	return done
}

func waitEntered(t *testing.T, agent *runKillingAgent, idTask int64) {
	t.Helper()
	select {
	case id := <-agent.entered:
		if id != idTask {
			t.Fatalf("agent started task %d, want %d", id, idTask)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("task %d never started", idTask)
	}
}

func waitDone(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s kept running", what)
	}
}

func assertRunning(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("%s was stopped", what)
	case <-time.After(200 * time.Millisecond):
	}
}

func isTaskActive(orchestrator *Orchestrator, idTask int64) bool {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()
	_, isActive := orchestrator.activeTasks[idTask]
	return isActive
}

func TestHandleEvent_CancelledStopsOnlyTheListedTasks(t *testing.T) {
	t.Run("the cancelled attempt stops, the newer one waiting for the slot runs", func(t *testing.T) {
		agent := newRunKillingAgent()
		t.Cleanup(func() { close(agent.release) })
		orchestrator, _, repo := resumeTestOrchestrator(t, agent)
		oldDone := startStage(orchestrator, resumeTestTask(repo, 100, 1))
		waitEntered(t, agent, 100)
		newDone := startStage(orchestrator, resumeTestTask(repo, 101, 2))
		waitUntil(t, "the newer attempt is queued", func() bool { return isTaskActive(orchestrator, 101) })

		if err := orchestrator.HandleEvent(cancelledEvent(200, `{"idsTask":[100]}`)); err != nil {
			t.Fatalf("HandleEvent(cancelled): %v", err)
		}

		waitDone(t, oldDone, "the cancelled attempt")
		waitEntered(t, agent, 101)
		assertRunning(t, newDone, "the newer attempt")
	})

	t.Run("a listed attempt waiting for the slot does not stop the running one", func(t *testing.T) {
		agent := newRunKillingAgent()
		t.Cleanup(func() { close(agent.release) })
		orchestrator, _, repo := resumeTestOrchestrator(t, agent)
		runningDone := startStage(orchestrator, resumeTestTask(repo, 101, 2))
		waitEntered(t, agent, 101)
		waitingDone := startStage(orchestrator, resumeTestTask(repo, 100, 1))
		waitUntil(t, "the listed attempt is queued", func() bool { return isTaskActive(orchestrator, 100) })

		if err := orchestrator.HandleEvent(cancelledEvent(200, `{"idsTask":[100]}`)); err != nil {
			t.Fatalf("HandleEvent(cancelled): %v", err)
		}

		waitDone(t, waitingDone, "the cancelled queued attempt")
		assertRunning(t, runningDone, "the running attempt")
	})

	t.Run("an event without idsTask stops every attempt of the run", func(t *testing.T) {
		agent := newRunKillingAgent()
		t.Cleanup(func() { close(agent.release) })
		orchestrator, _, repo := resumeTestOrchestrator(t, agent)
		runningDone := startStage(orchestrator, resumeTestTask(repo, 100, 1))
		waitEntered(t, agent, 100)
		queuedDone := startStage(orchestrator, resumeTestTask(repo, 101, 2))
		waitUntil(t, "the second attempt is queued", func() bool { return isTaskActive(orchestrator, 101) })

		if err := orchestrator.HandleEvent(cancelledEvent(200, `{}`)); err != nil {
			t.Fatalf("HandleEvent(cancelled): %v", err)
		}

		waitDone(t, runningDone, "the running attempt")
		waitDone(t, queuedDone, "the queued attempt")
	})
}

func TestRunStage_FinishedAttemptIsGoneBeforeTheNextTakesTheSlot(t *testing.T) {
	agent := newRunKillingAgent()
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	firstDone := startStage(orchestrator, resumeTestTask(repo, 100, 1))
	waitEntered(t, agent, 100)
	secondDone := startStage(orchestrator, resumeTestTask(repo, 101, 2))
	waitUntil(t, "the second attempt is queued", func() bool { return isTaskActive(orchestrator, 101) })

	agent.release <- struct{}{} // only the first attempt returns
	waitEntered(t, agent, 101)
	if isTaskActive(orchestrator, 100) {
		t.Error("the finished attempt was still listed as active while the next one held the slot")
	}
	waitDone(t, firstDone, "the finished attempt")

	close(agent.release)
	waitDone(t, secondDone, "the second attempt")
}
