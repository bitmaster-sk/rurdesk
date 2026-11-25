package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/rs/zerolog/log"
)

var retryBackoffs = []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}

type Dispatcher struct {
	agentRunRepo *repository.AgentRunRepository
	agentGwRepo  *repository.AgentGatewayRepository
	wikiAgent    *service.WikiAgentService
	kinds        *RunKinds
	gwClient     *GatewayClient
}

func NewDispatcher(
	agentRunRepo *repository.AgentRunRepository,
	agentGwRepo *repository.AgentGatewayRepository,
	wikiAgent *service.WikiAgentService,
	kinds *RunKinds,
	gwClient *GatewayClient,
) *Dispatcher {
	return &Dispatcher{
		agentRunRepo: agentRunRepo,
		agentGwRepo:  agentGwRepo,
		wikiAgent:    wikiAgent,
		kinds:        kinds,
		gwClient:     gwClient,
	}
}

// DispatchStage fires the stage_execute webhook of an activated task. A task that cannot be
// delivered interrupts its run with dispatch_failed.
func (d *Dispatcher) DispatchStage(_ context.Context, dispatch *Dispatch) {
	go func() {
		bgCtx := context.Background()
		kind := d.kinds.For(dispatch.Run)
		if err := d.deliverStage(bgCtx, kind, dispatch); err != nil {
			log.Error().Err(err).Int64("idTask", dispatch.Task.IdTask).Msg("dispatching stage_execute")
			if err := kind.Interrupt(bgCtx, dispatch.Run, dispatch.Task, InterruptDispatchFailed, err.Error()); err != nil {
				log.Error().Err(err).Int64("idTask", dispatch.Task.IdTask).Msg("interrupting an undeliverable stage")
			}
			return
		}
		kind.Started(bgCtx, dispatch)
	}()
}

func (d *Dispatcher) deliverStage(ctx context.Context, kind RunKind, dispatch *Dispatch) error {
	payload, wikiReads, err := kind.Payload(ctx, dispatch)
	if err != nil {
		return err
	}
	run := dispatch.Run
	event := WebhookEvent{
		IdRun:       run.IdRun,
		IdProject:   run.IdProject,
		IdIssue:     run.IdIssue,
		IdUserAgent: run.IdUserAgent,
		Event:       "stage_execute",
		Payload:     payload,
	}
	if err := d.DispatchEvent(ctx, run, event); err != nil {
		return fmt.Errorf("dispatch failed: %w", err)
	}
	if err := d.wikiAgent.RecordReads(ctx, wikiReads); err != nil {
		log.Error().Err(err).Int64("idTask", dispatch.Task.IdTask).Msg("recording wiki reads of the prompt")
	}
	return nil
}

// cancelledPayload lists the tasks the tracker cancelled.
type cancelledPayload struct {
	IdsTask []int64 `json:"idsTask"`
}

// DispatchCancelled tells the gateway to stop the cancelled tasks of the run. Only the listed tasks
// stop: a newer attempt of the same run, dispatched before this event arrives, keeps running.
func (d *Dispatcher) DispatchCancelled(ctx context.Context, run *model.AgentRun, idsTask []int64) error {
	if idsTask == nil {
		// null would make the gateway stop every task of the run.
		idsTask = []int64{}
	}
	event := WebhookEvent{
		IdRun:       run.IdRun,
		IdProject:   run.IdProject,
		IdIssue:     run.IdIssue,
		IdUserAgent: run.IdUserAgent,
		Event:       "cancelled",
		Payload:     cancelledPayload{IdsTask: idsTask},
	}
	return d.DispatchEvent(ctx, run, event)
}

// DispatchRunDiscarded tells the gateway that Restart replaced the run, so it
// stops the run's stage and removes its worktree.
func (d *Dispatcher) DispatchRunDiscarded(ctx context.Context, run *model.AgentRun) error {
	event := WebhookEvent{
		IdRun:       run.IdRun,
		IdProject:   run.IdProject,
		IdIssue:     run.IdIssue,
		IdUserAgent: run.IdUserAgent,
		Event:       "run_discarded",
		Payload:     map[string]any{},
	}
	return d.DispatchEvent(ctx, run, event)
}

func (d *Dispatcher) DispatchEvent(ctx context.Context, run *model.AgentRun, event WebhookEvent) error {
	gateway, err := d.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil || gateway == nil {
		return fmt.Errorf("no gateway configured for bot %d", run.IdUserAgent)
	}

	seq, err := d.agentRunRepo.CountEvents(ctx, run.IdRun)
	if err != nil {
		seq = 0
	}
	event.Sequence = seq + 1

	return d.gwClient.DeliverEvent(ctx, gateway, event, retryBackoffs)
}

func retryWithBackoff(ctx context.Context, fn func() error, backoffs []time.Duration) error {
	var lastErr error
	for i, delay := range backoffs {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if i < len(backoffs)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("all retries exhausted: %w", lastErr)
}
