package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// BroadcastRunUpdate sends a SubjectAgentRun notice with a self-contained
// snapshot (run, events, derived stage timeline) so clients patch their store
// without a refetch. Events/tasks loads are best-effort — on error the notice
// still goes out with an empty slice, so a DB hiccup doesn't swallow the phase
// update. Scoped to project members only.
func BroadcastRunUpdate(
	ctx context.Context,
	notifier *notify.Notifier,
	projectRepo *repository.ProjectRepository,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	run *model.AgentRun,
) {
	if notifier == nil || run == nil {
		return
	}
	events, err := runRepo.LoadEvents(ctx, run.IdRun)
	if err != nil {
		log.Debug().Err(err).Int64("idRun", run.IdRun).Msg("BroadcastRunUpdate: events load failed, sending without")
		events = nil
	}
	tasks, err := taskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		log.Debug().Err(err).Int64("idRun", run.IdRun).Msg("BroadcastRunUpdate: tasks load failed, sending without")
		tasks = nil
	}

	var idsUser []int64
	if projectRepo != nil {
		members, err := projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
		if err == nil {
			idsUser = make([]int64, len(members))
			for i, m := range members {
				idsUser[i] = m.IdUser
			}
		}
	}

	notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectAgentRun,
		Action:  notify.ActionUpdate,
		Payload: BuildRunSnapshot(run, events, tasks),
	}
}

// issueLoader / projectMemberLoader are the narrow seams BroadcastIssueUpdate
// depends on; *repository.IssueRepository and *repository.ProjectRepository
// satisfy them, and tests can inject fakes without a DB.
type issueLoader interface {
	LoadIssue(ctx context.Context, f *repository.LoadIssueFilter) (*model.Issue, error)
}

type projectMemberLoader interface {
	LoadProjectsMembers(ctx context.Context, idsProject []int64) ([]*model.User, error)
}

// BroadcastIssueUpdate loads the issue and broadcasts it as a SubjectIssue
// notice so clients viewing the issue detail patch their copy without a
// refetch. Used after agent flows mutate the issue out-of-band (PR link,
// phase→state mirror, merge poller transitions), which would otherwise leave
// the detail page stale.
//
// Scoped to project members via IdsUser: a bare notice (empty IdsUser, IdUser
// 0) would fan out to every session, leaking the issue across projects — so
// unresolved membership skips the send instead of falling back to a global
// broadcast. Best-effort: an issue-load failure also skips the send (the run
// notice already went out).
func BroadcastIssueUpdate(
	ctx context.Context,
	notifier *notify.Notifier,
	issueRepo issueLoader,
	projectRepo projectMemberLoader,
	idIssue int64,
) {
	BroadcastIssueNotice(ctx, notifier, issueRepo, projectRepo, idIssue, notify.ActionUpdate)
}

// BroadcastIssueNotice is BroadcastIssueUpdate with an explicit action, used by
// REST issue endpoints so human-driven create/update reaches other clients'
// open views live (table row wash, kanban card fly).
func BroadcastIssueNotice(
	ctx context.Context,
	notifier *notify.Notifier,
	issueRepo issueLoader,
	projectRepo projectMemberLoader,
	idIssue int64,
	action notify.NoticeAction,
) {
	if notifier == nil {
		return
	}
	issue, err := issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err != nil || issue == nil {
		log.Debug().Err(err).Int64("idIssue", idIssue).Msg("BroadcastIssueNotice: issue load failed, skipping")
		return
	}
	BroadcastIssueSnapshot(ctx, notifier, projectRepo, issue, action)
}

// BroadcastIssueSnapshot broadcasts an already-loaded issue to the project's
// members. Used for deletes, where the row is gone by the time the notice
// goes out and only the pre-delete snapshot remains.
func BroadcastIssueSnapshot(
	ctx context.Context,
	notifier *notify.Notifier,
	projectRepo projectMemberLoader,
	issue *model.Issue,
	action notify.NoticeAction,
) {
	if notifier == nil || issue == nil {
		return
	}
	members, err := projectRepo.LoadProjectsMembers(ctx, []int64{issue.IdProject})
	if err != nil || len(members) == 0 {
		log.Debug().Err(err).Int64("idIssue", issue.IdIssue).Int64("idProject", issue.IdProject).
			Msg("BroadcastIssueSnapshot: no project members resolved, skipping to avoid global broadcast")
		return
	}
	idsUser := make([]int64, len(members))
	for i, m := range members {
		idsUser[i] = m.IdUser
	}
	notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectIssue,
		Action:  action,
		Payload: issue,
	}
}

// FailRun transitions a non-terminal run to failed and broadcasts the
// snapshot so the card surfaces the error and Continue/Restart. No-op if
// already terminal. Used for gateway-loss recovery (restart, stale
// heartbeat, crash) where an in-flight stage was orphaned.
func FailRun(
	ctx context.Context,
	notifier *notify.Notifier,
	projectRepo *repository.ProjectRepository,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	idRun int64,
	reason string,
) {
	run, err := runRepo.LoadById(ctx, idRun)
	if err != nil || run == nil || constants.TerminalPhases[run.Phase] {
		return
	}
	updated, err := runRepo.TransitionPhase(ctx, idRun, run.Phase, constants.PhaseFailed, constants.ActorTypeSystem, nil, reason)
	if err != nil {
		log.Debug().Err(err).Int64("idRun", idRun).Msg("FailRun: transition skipped")
		return
	}
	if msg := reason; msg != "" {
		_ = runRepo.SetErrorMessage(ctx, idRun, msg)
		updated.ErrorMessage = &msg
	}
	BroadcastRunUpdate(ctx, notifier, projectRepo, runRepo, taskRepo, updated)
}
