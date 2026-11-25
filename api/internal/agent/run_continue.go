package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

// ContinueRun moves a failed or cancelled run back to where it stopped: a run stopped while it waited
// for a person waits again and the returned stage is empty, any other run is queued to redo the returned stage.
func ContinueRun(
	ctx context.Context,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	run *model.AgentRun,
	actorType string,
	idUser *int64,
	reason string,
) (*model.AgentRun, string, error) {
	if run.Phase != constants.PhaseFailed && run.Phase != constants.PhaseCancelled {
		return nil, "", errs.ErrRunContinueInvalid
	}
	events, err := runRepo.LoadEvents(ctx, run.IdRun)
	if err != nil {
		return nil, "", err
	}
	if stoppedIn := phaseStoppedIn(events, run.Phase); constants.PassivePhases[stoppedIn] {
		updated, err := runRepo.TransitionPhase(ctx, run.IdRun, run.Phase, stoppedIn, actorType, idUser, reason)
		return updated, "", err
	}
	tasks, err := taskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		return nil, "", err
	}
	stage, err := ResolveNextStage(run, tasks)
	if err != nil || stage == "" {
		return nil, "", errs.ErrNoStageToContinue
	}
	updated, err := runRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseQueued, actorType, idUser, reason)
	if err != nil {
		return nil, "", err
	}
	return updated, stage, nil
}

// phaseStoppedIn returns the phase the run left for its terminal phase, "" when no event records it.
func phaseStoppedIn(events []*model.AgentRunEvent, terminalPhase string) string {
	stoppedIn := ""
	for _, event := range events {
		if event.ToPhase != nil && *event.ToPhase == terminalPhase && event.FromPhase != nil {
			stoppedIn = *event.FromPhase
		}
	}
	return stoppedIn
}
