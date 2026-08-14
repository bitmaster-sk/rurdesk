package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type Task struct {
	Name       string
	Interval   time.Duration
	RunOnStart bool
	Run        func(ctx context.Context) error
}

type Scheduler struct {
	tasks []Task
}

func New(tasks ...Task) *Scheduler {
	return &Scheduler{tasks: tasks}
}

func (s *Scheduler) Register(task Task) {
	s.tasks = append(s.tasks, task)
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, task := range s.tasks {
		go s.loop(ctx, task)
	}
	<-ctx.Done()
}

func (s *Scheduler) loop(ctx context.Context, task Task) {
	if task.RunOnStart {
		s.runOnce(ctx, task)
	}
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx, task)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context, task Task) {
	if err := task.Run(ctx); err != nil {
		log.Error().Err(err).Str("task", task.Name).Msg("scheduled task failed")
	}
}
