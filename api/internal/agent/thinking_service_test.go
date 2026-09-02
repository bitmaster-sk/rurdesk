package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	storedBytes int
	isTruncated bool
	appended    []model.AgentThinkingEvents
	truncated   int
	orphans     []int64
	events      map[int64]model.AgentThinkingEvents
	unreadable  int64
	compactedTo map[int64]string
	blobs       map[int64][]byte
	loadedBlob  []byte
	loadedTail  string
	loadErr     error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		events:      map[int64]model.AgentThinkingEvents{},
		compactedTo: map[int64]string{},
		blobs:       map[int64][]byte{},
	}
}

func (f *fakeStore) StoredSize(ctx context.Context, idTask int64) (int, bool, error) {
	return f.storedBytes, f.isTruncated, nil
}

func (f *fakeStore) Append(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) error {
	f.appended = append(f.appended, events)
	return nil
}

func (f *fakeStore) MarkTruncated(ctx context.Context, idTask int64, seq int) error {
	f.truncated++
	return nil
}

func (f *fakeStore) LoadEvents(ctx context.Context, idTask int64) (model.AgentThinkingEvents, error) {
	if f.unreadable == idTask {
		return nil, errors.New("event row is unreadable")
	}
	return f.events[idTask], nil
}

func (f *fakeStore) Compact(ctx context.Context, idTask int64, blob []byte, tail string) error {
	f.compactedTo[idTask] = tail
	f.blobs[idTask] = blob
	return nil
}

func (f *fakeStore) LoadCompacted(ctx context.Context, idRun int64, stage string) ([]byte, string, error) {
	return f.loadedBlob, f.loadedTail, f.loadErr
}

func (f *fakeStore) OrphanedTaskIds(ctx context.Context) ([]int64, error) {
	return f.orphans, nil
}

type fakeTasks struct {
	heartbeats int
	beatErr    error
}

func (f *fakeTasks) LoadById(ctx context.Context, idTask int64) (*model.AgentTask, error) {
	return &model.AgentTask{IdTask: idTask, IdRun: 9, Stage: "implementation"}, nil
}

func (f *fakeTasks) RecordHeartbeat(ctx context.Context, idTask int64) error {
	f.heartbeats++
	return f.beatErr
}

type fakeRuns struct{}

func (f *fakeRuns) LoadById(ctx context.Context, idRun int64) (*model.AgentRun, error) {
	return &model.AgentRun{IdRun: idRun, IdProject: 3}, nil
}

type fakeNotifier struct {
	notices    []*model.AgentThinkingNotice
	idsProject []int64
}

func (f *fakeNotifier) Broadcast(ctx context.Context, idProject int64, notice *model.AgentThinkingNotice) {
	f.notices = append(f.notices, notice)
	f.idsProject = append(f.idsProject, idProject)
}

const testThinkingMaxKb = 1024

type fakeSettings struct {
	isOn  bool
	maxKb int
}

func (f *fakeSettings) IsAgentThinkingPersisted() bool { return f.isOn }

func (f *fakeSettings) AgentThinkingMaxKb() int { return f.maxKb }

func newTestService(store *fakeStore, isPersisted bool) (*ThinkingService, *fakeTasks, *fakeNotifier) {
	tasks := &fakeTasks{}
	notifier := &fakeNotifier{}
	svc := NewThinkingService(store, tasks, &fakeRuns{}, notifier, &fakeSettings{isOn: isPersisted, maxKb: testThinkingMaxKb})
	return svc, tasks, notifier
}

func thinkingEvent(text string) model.AgentThinkingEvent {
	return model.AgentThinkingEvent{Kind: model.ThinkingKindThinking, Text: text, At: 1}
}

func TestThinkingService_CreateStoresBroadcastsAndBeats(t *testing.T) {
	store := newFakeStore()
	svc, tasks, notifier := newTestService(store, true)

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("a thought")}))

	assert.Equal(t, 1, tasks.heartbeats)
	require.Len(t, store.appended, 1)
	require.Len(t, notifier.notices, 1)
	assert.Equal(t, "implementation", notifier.notices[0].Stage)
	assert.Equal(t, int64(9), notifier.notices[0].IdRun)
	assert.Equal(t, int64(1), notifier.notices[0].IdTask)
	assert.Equal(t, []int64{3}, notifier.idsProject, "the batch goes to the run's project, never wider")
}

func TestThinkingService_CreateSkipsPersistenceWhenOff(t *testing.T) {
	store := newFakeStore()
	svc, _, notifier := newTestService(store, false)

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("a thought")}))

	assert.Empty(t, store.appended, "nothing is written while the switch is off")
	assert.Len(t, notifier.notices, 1, "the live feed still runs")
}

func TestThinkingService_CreateIgnoresEventsItDoesNotAccept(t *testing.T) {
	cases := map[string]model.AgentThinkingEvent{
		"forged truncation marker": {Kind: model.ThinkingKindTruncated, Text: "x"},
		"unknown kind":             {Kind: "shouting", Text: "x"},
		"empty":                    {Kind: model.ThinkingKindThinking, Text: "  "},
	}
	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			svc, tasks, notifier := newTestService(store, true)

			require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{event}))

			assert.Empty(t, store.appended)
			assert.Empty(t, notifier.notices)
			assert.Zero(t, tasks.heartbeats, "an empty batch is not a sign of life")
		})
	}
}

// A resend carries its original seq. The rows dedup on their key, but the tail
// and the broadcast would otherwise repeat.
func TestThinkingService_CreateDoesNotRepeatAReplay(t *testing.T) {
	store := newFakeStore()
	svc, _, notifier := newTestService(store, true)
	batch := model.AgentThinkingEvents{thinkingEvent("one thought")}

	require.NoError(t, svc.Create(context.Background(), 1, 1, batch))
	require.NoError(t, svc.Create(context.Background(), 1, 1, batch))

	assert.Len(t, store.appended, 2, "a resend still gets its rows, which dedup in the database")
	assert.Len(t, notifier.notices, 1, "the reader must not see the sentence twice")

	svc.Compact(context.Background(), 1)
	assert.Equal(t, "one thought", store.compactedTo[1])
}

func TestThinkingService_CreateStopsAtTheByteCap(t *testing.T) {
	store := newFakeStore()
	store.storedBytes = testThinkingMaxKb * 1024
	svc, _, _ := newTestService(store, true)

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("over budget")}))

	assert.Empty(t, store.appended)
	assert.Equal(t, 1, store.truncated)
}

func TestThinkingService_CapFollowsTheConfiguredSize(t *testing.T) {
	store := newFakeStore()
	store.storedBytes = 64 * 1024
	svc := NewThinkingService(store, &fakeTasks{}, &fakeRuns{}, &fakeNotifier{}, &fakeSettings{isOn: true, maxKb: 128})

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("within budget")}))
	assert.Len(t, store.appended, 1)

	store.storedBytes = 128 * 1024
	require.NoError(t, svc.Create(context.Background(), 1, 2, model.AgentThinkingEvents{thinkingEvent("over budget")}))
	assert.Len(t, store.appended, 1)
	assert.Equal(t, 1, store.truncated)
}

// Once a stage is marked truncated the cap latches: a smaller batch must not
// slip in behind the marker, or the replay reads as complete past that point.
func TestThinkingService_CreateLatchesOnceTruncated(t *testing.T) {
	store := newFakeStore()
	store.isTruncated = true
	svc, _, _ := newTestService(store, true)

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("tiny")}))

	assert.Empty(t, store.appended)
	assert.Zero(t, store.truncated, "the marker is written once, not once per dropped batch")
}

func TestThinkingService_CreateReturnsHeartbeatFailure(t *testing.T) {
	store := newFakeStore()
	svc, tasks, _ := newTestService(store, true)
	tasks.beatErr = errors.New("task is not active")

	err := svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("a thought")})

	require.Error(t, err)
	assert.Empty(t, store.appended)
}

func TestThinkingService_CompactWritesBlobAndTail(t *testing.T) {
	store := newFakeStore()
	store.events[1] = model.AgentThinkingEvents{thinkingEvent("kept in full")}
	svc, _, _ := newTestService(store, true)
	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("kept in full")}))

	svc.Compact(context.Background(), 1)

	assert.Equal(t, "kept in full", store.compactedTo[1])
	assert.NotEmpty(t, store.blobs[1])
}

func TestThinkingService_CompactSkipsATaskWithNothingToKeep(t *testing.T) {
	store := newFakeStore()
	svc, _, _ := newTestService(store, true)

	svc.Compact(context.Background(), 1)

	assert.NotContains(t, store.compactedTo, int64(1))
}

func TestThinkingService_CompactTakesEveryTaskGiven(t *testing.T) {
	store := newFakeStore()
	svc, _, _ := newTestService(store, true)
	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent("first")}))
	require.NoError(t, svc.Create(context.Background(), 2, 1, model.AgentThinkingEvents{thinkingEvent("second")}))

	svc.Compact(context.Background(), 1, 2)

	assert.Equal(t, "first", store.compactedTo[1])
	assert.Equal(t, "second", store.compactedTo[2])
}

func TestThinkingService_CompactOrphaned(t *testing.T) {
	store := newFakeStore()
	store.orphans = []int64{5}
	store.events[5] = model.AgentThinkingEvents{
		{Kind: model.ThinkingKindThinking, Text: "thinking kept"},
		{Kind: model.ThinkingKindTool, Tool: "secret__tool_output"},
	}
	svc, _, _ := newTestService(store, true)

	compacted, err := svc.CompactOrphaned(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, compacted)
	assert.Equal(t, "thinking kept", store.compactedTo[5], "tool output must stay out of the tail")
}

func TestThinkingService_CompactOrphanedSkipsATaskItCannotRead(t *testing.T) {
	store := newFakeStore()
	store.orphans = []int64{5, 6}
	store.unreadable = 5
	store.events[6] = model.AgentThinkingEvents{thinkingEvent("the task behind the bad one")}
	svc, _, _ := newTestService(store, true)

	compacted, err := svc.CompactOrphaned(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, compacted)
	assert.Equal(t, "the task behind the bad one", store.compactedTo[6])
}

func TestThinkingService_LoadForStageReplaysTheBlob(t *testing.T) {
	events := model.AgentThinkingEvents{thinkingEvent("the whole story")}
	blob, err := events.Gzip()
	require.NoError(t, err)
	store := newFakeStore()
	store.loadedBlob = blob
	svc, _, _ := newTestService(store, true)

	res, err := svc.LoadForStage(context.Background(), 9, "implementation")

	require.NoError(t, err)
	assert.True(t, res.IsComplete)
	assert.Equal(t, events, res.Events)
}

func TestThinkingService_LoadForStageFallsBackToTheTail(t *testing.T) {
	cases := map[string][]byte{
		"no blob stored": nil,
		"damaged blob":   []byte("not gzip"),
	}
	for name, blob := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			store.loadedBlob = blob
			store.loadedTail = "the last thoughts"
			svc, _, _ := newTestService(store, true)

			res, err := svc.LoadForStage(context.Background(), 9, "implementation")

			require.NoError(t, err)
			assert.False(t, res.IsComplete, "a tail must not pass as the full thinking")
			require.Len(t, res.Events, 1)
			assert.Equal(t, "the last thoughts", res.Events[0].Text)
		})
	}
}

func TestThinkingService_LoadForStageOfAStageWithNothing(t *testing.T) {
	svc, _, _ := newTestService(newFakeStore(), true)

	res, err := svc.LoadForStage(context.Background(), 9, "implementation")

	require.NoError(t, err)
	assert.False(t, res.IsComplete)
	assert.Empty(t, res.Events)
	assert.NotNil(t, res.Events, "the feed renders an empty list, never a null")
}

func TestThinkingService_LoadForStageSurfacesAStoreFailure(t *testing.T) {
	store := newFakeStore()
	store.loadErr = errors.New("database is down")
	svc, _, _ := newTestService(store, true)

	_, err := svc.LoadForStage(context.Background(), 9, "implementation")

	require.Error(t, err)
}

func TestThinkingService_TailIsCappedAcrossBatches(t *testing.T) {
	store := newFakeStore()
	svc, _, _ := newTestService(store, false)

	require.NoError(t, svc.Create(context.Background(), 1, 1, model.AgentThinkingEvents{thinkingEvent(strings.Repeat("a", 4000))}))
	require.NoError(t, svc.Create(context.Background(), 1, 2, model.AgentThinkingEvents{thinkingEvent("the last thought")}))
	store.events[1] = nil
	svc.Compact(context.Background(), 1)

	tail := store.compactedTo[1]
	assert.LessOrEqual(t, len(tail), thinkingTailBytes)
	assert.Contains(t, tail, "the last thought")
}
