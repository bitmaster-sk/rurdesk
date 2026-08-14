package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within the deadline")
}

func TestSchedulerRunsATaskOnStartAndOnEveryTick(t *testing.T) {
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go New(Task{
		Name:       "counter",
		Interval:   5 * time.Millisecond,
		RunOnStart: true,
		Run: func(context.Context) error {
			runs.Add(1)
			return nil
		},
	}).Start(ctx)

	waitFor(t, func() bool { return runs.Load() >= 3 })
}

func TestSchedulerKeepsRunningAfterATaskFails(t *testing.T) {
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go New(Task{
		Name:       "failing",
		Interval:   5 * time.Millisecond,
		RunOnStart: true,
		Run: func(context.Context) error {
			runs.Add(1)
			return errors.New("boom")
		},
	}).Start(ctx)

	waitFor(t, func() bool { return runs.Load() >= 3 })
}

func TestSchedulerStopsWhenTheContextIsCancelled(t *testing.T) {
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	go New(Task{
		Name:     "counter",
		Interval: 5 * time.Millisecond,
		Run: func(context.Context) error {
			runs.Add(1)
			return nil
		},
	}).Start(ctx)

	waitFor(t, func() bool { return runs.Load() >= 2 })
	cancel()
	time.Sleep(20 * time.Millisecond)

	stopped := runs.Load()
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, stopped, runs.Load(), "no task may run after the context is cancelled")
}

func TestSchedulerRunsEveryRegisteredTask(t *testing.T) {
	var first, second atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(Task{
		Name:       "first",
		Interval:   time.Hour,
		RunOnStart: true,
		Run: func(context.Context) error {
			first.Add(1)
			return nil
		},
	})
	s.Register(Task{
		Name:       "second",
		Interval:   time.Hour,
		RunOnStart: true,
		Run: func(context.Context) error {
			second.Add(1)
			return nil
		},
	})
	go s.Start(ctx)

	waitFor(t, func() bool { return first.Load() == 1 && second.Load() == 1 })
}
