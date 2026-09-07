package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/rs/zerolog/log"
)

type thinkingStore interface {
	StoredSize(ctx context.Context, idTask int64) (storedBytes int, isTruncated bool, err error)
	Append(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) error
	MarkTruncated(ctx context.Context, idTask int64, seq int) error
	LoadEventsByTask(ctx context.Context, idTask int64) (model.AgentThinkingEvents, error)
	LoadEventsByStage(ctx context.Context, idRun int64, stage string) (events model.AgentThinkingEvents, idTask int64, lastSeq int, err error)
	Compact(ctx context.Context, idTask int64, blob []byte, tail string) error
	LoadCompacted(ctx context.Context, idRun int64, stage string) (blob []byte, tail string, err error)
	OrphanedTaskIds(ctx context.Context) ([]int64, error)
}

type thinkingTaskLoader interface {
	LoadById(ctx context.Context, idTask int64) (*model.AgentTask, error)
	RecordHeartbeat(ctx context.Context, idTask int64) error
}

type thinkingRunLoader interface {
	LoadById(ctx context.Context, idRun int64) (*model.AgentRun, error)
}

type thinkingNotifier interface {
	Broadcast(ctx context.Context, idProject int64, notice *model.AgentThinkingNotice)
}

type thinkingSettings interface {
	IsAgentThinkingPersisted() bool
	AgentThinkingMaxKb() int
}

type ThinkingService struct {
	store    thinkingStore
	tasks    thinkingTaskLoader
	runs     thinkingRunLoader
	notifier thinkingNotifier
	settings thinkingSettings
	tails    *thinkingTails
}

func NewThinkingService(
	store thinkingStore,
	tasks thinkingTaskLoader,
	runs thinkingRunLoader,
	notifier thinkingNotifier,
	settings thinkingSettings,
) *ThinkingService {
	return &ThinkingService{
		store:    store,
		tasks:    tasks,
		runs:     runs,
		notifier: notifier,
		settings: settings,
		tails:    newThinkingTails(),
	}
}

// Create takes one batch of thinking events from the gateway: it reports the
// task heartbeat, stores the batch when persistence is on, and broadcasts it
// live. A resend under an already seen seq is stored again but is neither
// appended to the tail nor broadcast a second time.
func (s *ThinkingService) Create(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) error {
	accepted := events.Accepted()
	if len(accepted) == 0 {
		return nil
	}
	if err := s.tasks.RecordHeartbeat(ctx, idTask); err != nil {
		return err
	}

	isReplay := s.tails.HasSeenSeq(idTask, seq)

	if s.settings.IsAgentThinkingPersisted() {
		if err := s.persist(ctx, idTask, seq, accepted); err != nil {
			return err
		}
	}

	// Read before the write above, so a resend after a failed write still gets
	// its rows.
	if isReplay {
		return nil
	}
	for _, event := range accepted {
		if event.Kind == model.ThinkingKindThinking {
			s.tails.Append(idTask, event.Text)
		}
	}
	s.tails.RecordSeq(idTask, seq)
	s.broadcast(ctx, idTask, seq, accepted)
	return nil
}

// persist appends the batch to the task's stored events, or marks the task
// truncated once the batch would take it past the byte cap.
func (s *ThinkingService) persist(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) error {
	storedBytes, isTruncated, err := s.store.StoredSize(ctx, idTask)
	if err != nil {
		return err
	}
	// The cap latches: a smaller batch let in behind the marker would make the
	// replay read as complete past the point where it stopped being so.
	if isTruncated {
		return nil
	}
	batchBytes := 0
	for _, event := range events {
		batchBytes += event.Size()
	}
	if storedBytes+batchBytes > s.settings.AgentThinkingMaxKb()*1024 {
		return s.store.MarkTruncated(ctx, idTask, seq)
	}
	return s.store.Append(ctx, idTask, seq, events)
}

func (s *ThinkingService) broadcast(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) {
	task, err := s.tasks.LoadById(ctx, idTask)
	if err != nil {
		log.Warn().Err(err).Int64("idTask", idTask).Msg("loading task to broadcast thinking")
		return
	}
	run, err := s.runs.LoadById(ctx, task.IdRun)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", task.IdRun).Msg("loading run to broadcast thinking")
		return
	}
	s.notifier.Broadcast(ctx, run.IdProject, &model.AgentThinkingNotice{
		IdRun:  run.IdRun,
		IdTask: idTask,
		Stage:  task.Stage,
		Seq:    seq,
		Events: events,
	})
}

// Compact folds each task's live event rows into one compressed blob plus a
// tail, and drops the rows. Best-effort: a task that cannot be compacted is
// logged and skipped, never fails the caller's request.
func (s *ThinkingService) Compact(ctx context.Context, idsTask ...int64) {
	for _, idTask := range idsTask {
		tail := s.tails.Take(idTask)

		events, err := s.store.LoadEventsByTask(ctx, idTask)
		if err != nil {
			log.Warn().Err(err).Int64("idTask", idTask).Msg("loading thinking events for compaction")
			continue
		}
		if len(events) == 0 && tail == "" {
			continue
		}

		var blob []byte
		if len(events) > 0 {
			blob, err = events.Gzip()
			if err != nil {
				log.Warn().Err(err).Int64("idTask", idTask).Msg("compressing thinking for compaction")
				continue
			}
		}
		if err := s.store.Compact(ctx, idTask, blob, tail); err != nil {
			log.Warn().Err(err).Int64("idTask", idTask).Msg("compacting thinking")
		}
	}
}

// CompactOrphaned sweeps tasks that left rows behind without ever reaching the
// complete path — cancelled runs and stages the stale sweep failed.
func (s *ThinkingService) CompactOrphaned(ctx context.Context) (int, error) {
	idsTask, err := s.store.OrphanedTaskIds(ctx)
	if err != nil {
		return 0, err
	}
	compacted := 0
	for _, idTask := range idsTask {
		events, err := s.store.LoadEventsByTask(ctx, idTask)
		if err != nil {
			log.Warn().Err(err).Int64("idTask", idTask).Msg("loading orphaned thinking")
			continue
		}
		blob, err := events.Gzip()
		if err != nil {
			log.Warn().Err(err).Int64("idTask", idTask).Msg("compressing orphaned thinking")
			continue
		}
		if err := s.store.Compact(ctx, idTask, blob, tailFromEvents(events)); err != nil {
			log.Warn().Err(err).Int64("idTask", idTask).Msg("compacting orphaned thinking")
			continue
		}
		compacted++
	}
	return compacted, nil
}

func (s *ThinkingService) SweepTails() int {
	return s.tails.Sweep(thinkingTailMaxAge)
}

// LoadForStage returns the stage's recorded thinking. An unreadable blob falls
// back to the tail rather than failing: a damaged replay is still worth the
// last thoughts.
func (s *ThinkingService) LoadForStage(ctx context.Context, idRun int64, stage string) (model.AgentThinkingRes, error) {
	blob, tail, err := s.store.LoadCompacted(ctx, idRun, stage)
	if err != nil {
		return model.AgentThinkingRes{}, err
	}
	if len(blob) == 0 {
		// A stage that is still running has rows instead of a blob. Replaying
		// them is what lets a reloaded page rejoin the stream where it stands.
		events, idTask, lastSeq, err := s.store.LoadEventsByStage(ctx, idRun, stage)
		if err != nil {
			return model.AgentThinkingRes{}, err
		}
		if len(events) == 0 {
			return tailThinkingRes(idRun, stage, tail), nil
		}
		return model.AgentThinkingRes{
			IdRun:   idRun,
			IdTask:  idTask,
			Stage:   stage,
			Events:  events,
			LastSeq: lastSeq,
		}, nil
	}
	var events model.AgentThinkingEvents
	if err := events.Gunzip(blob); err != nil {
		log.Warn().Err(err).Int64("idRun", idRun).Str("stage", stage).Msg("reading thinking blob")
		return tailThinkingRes(idRun, stage, tail), nil
	}
	return model.AgentThinkingRes{IdRun: idRun, Stage: stage, Events: events, IsComplete: true}, nil
}

func tailThinkingRes(idRun int64, stage string, tail string) model.AgentThinkingRes {
	res := model.AgentThinkingRes{IdRun: idRun, Stage: stage, Events: model.AgentThinkingEvents{}}
	if tail != "" {
		res.Events = append(res.Events, model.AgentThinkingEvent{
			Kind: model.ThinkingKindThinking,
			Text: tail,
		})
	}
	return res
}
