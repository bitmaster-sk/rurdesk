package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify/notifytest"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMappingLoader struct {
	mapping *model.WorkflowEventMapping
	err     error
	events  []string
}

func (f *fakeMappingLoader) LoadMapping(_ context.Context, _ int64, event string) (*model.WorkflowEventMapping, error) {
	f.events = append(f.events, event)
	return f.mapping, f.err
}

type fakeStateLoader struct {
	state *model.State
	err   error
}

func (f fakeStateLoader) LoadState(_ context.Context, _, _ int64) (*model.State, error) {
	return f.state, f.err
}

type fakeIssueStateStore struct {
	issue        *model.Issue
	updateErr    error
	writtenState *int64
}

func (f *fakeIssueStateStore) LoadIssue(_ context.Context, _ *repository.LoadIssueFilter) (*model.Issue, error) {
	return f.issue, nil
}

func (f *fakeIssueStateStore) UpdateIssueState(_ context.Context, _ int64, idState int64) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.writtenState = &idState
	return nil
}

type fakeHistoryRecorder struct {
	changes []model.IssueHistoryChange
	err     error
}

func (f *fakeHistoryRecorder) RecordSystem(_ context.Context, _ int64, changes []model.IssueHistoryChange) error {
	if f.err != nil {
		return f.err
	}
	f.changes = append(f.changes, changes...)
	return nil
}

type fakeTx struct {
	pgx.Tx
}

func txContext() context.Context {
	return extctx.WithTx(context.Background(), fakeTx{})
}

func idPtr(id int64) *int64 { return &id }

func newMirrorFixture(
	mapping *model.WorkflowEventMapping,
	state *model.State,
) (*PhaseStateTransitioner, *fakeIssueStateStore, *notifytest.Recorder) {
	store := &fakeIssueStateStore{issue: &model.Issue{IdIssue: 42, IdProject: 7}}
	notifier := &notifytest.Recorder{}
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{mapping: mapping},
		store,
		fakeStateLoader{state: state},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
		&fakeHistoryRecorder{},
		nil,
	)
	return mirror, store, notifier
}

func TestTransition_WritesStateAndBroadcasts(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(
		&model.WorkflowEventMapping{IdState: idPtr(99)},
		&model.State{IdState: 99, Name: "In QA"},
	)

	mirror.Transition(txContext(), 7, 42, constants.PhasePrOpen)

	require.NotNil(t, store.writtenState, "the mapped state must be written to the issue")
	assert.Equal(t, int64(99), *store.writtenState)

	require.Len(t, notifier.Notices(), 1, "a state change must reach the project over the websocket")
	notice := notifier.Notices()[0]
	assert.Equal(t, notify.SubjectIssue, notice.Subject)
	assert.Equal(t, []int64{10}, notice.IdsUser)
}

func TestTransition_NoMappingChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(nil, nil)

	mirror.Transition(txContext(), 7, 42, constants.PhaseQueued)

	assert.Nil(t, store.writtenState, "an unmapped event must leave the issue state alone")
	assert.Empty(t, notifier.Notices(), "an unmapped event must not broadcast")
}

func TestTransition_NullMappedStateChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(&model.WorkflowEventMapping{IdState: nil}, nil)

	mirror.Transition(txContext(), 7, 42, constants.PhaseDone)

	assert.Nil(t, store.writtenState)
	assert.Empty(t, notifier.Notices())
}

func TestTransition_MissingStateChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(&model.WorkflowEventMapping{IdState: idPtr(99)}, nil)

	mirror.Transition(txContext(), 7, 42, constants.PhaseDone)

	assert.Nil(t, store.writtenState, "a mapping pointing at a deleted state must not be applied")
	assert.Empty(t, notifier.Notices())
}

func TestTransition_MappingLoadErrorChangesNothing(t *testing.T) {
	store := &fakeIssueStateStore{issue: &model.Issue{IdIssue: 42, IdProject: 7}}
	notifier := &notifytest.Recorder{}
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{err: errors.New("db down")},
		store,
		fakeStateLoader{state: &model.State{IdState: 99}},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
		&fakeHistoryRecorder{},
		nil,
	)

	mirror.Transition(txContext(), 7, 42, constants.PhaseFailed)

	assert.Nil(t, store.writtenState)
	assert.Empty(t, notifier.Notices())
}

func TestTransition_WriteFailureDoesNotBroadcast(t *testing.T) {
	store := &fakeIssueStateStore{
		issue:     &model.Issue{IdIssue: 42, IdProject: 7},
		updateErr: errors.New("write failed"),
	}
	notifier := &notifytest.Recorder{}
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{mapping: &model.WorkflowEventMapping{IdState: idPtr(99)}},
		store,
		fakeStateLoader{state: &model.State{IdState: 99, Name: "In QA"}},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
		&fakeHistoryRecorder{},
		nil,
	)

	mirror.Transition(txContext(), 7, 42, constants.PhaseDone)

	assert.Empty(t, notifier.Notices(), "a failed write must not announce a state it did not make")
}

func TestTransition_LooksUpTheEventItWasGiven(t *testing.T) {
	loader := &fakeMappingLoader{}
	mirror := NewPhaseStateTransitioner(
		loader,
		&fakeIssueStateStore{},
		fakeStateLoader{},
		fakeMemberLoader{},
		&notifytest.Recorder{},
		&fakeHistoryRecorder{},
		nil,
	)

	mirror.Transition(txContext(), 7, 42, constants.PhaseAwaitingInput)

	assert.Equal(t, []string{constants.PhaseAwaitingInput}, loader.events)
}

func newHistoryFixture(currentState *int64, history *fakeHistoryRecorder) (*PhaseStateTransitioner, *fakeIssueStateStore, *notifytest.Recorder) {
	store := &fakeIssueStateStore{issue: &model.Issue{IdIssue: 42, IdProject: 7, IdState: currentState}}
	notifier := &notifytest.Recorder{}
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{mapping: &model.WorkflowEventMapping{IdState: idPtr(99)}},
		store,
		fakeStateLoader{state: &model.State{IdState: 99, Name: "In QA"}},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
		history,
		nil,
	)
	return mirror, store, notifier
}

func TestTransition_RecordsTheStateChangeAsSystemHistory(t *testing.T) {
	history := &fakeHistoryRecorder{}
	mirror, _, _ := newHistoryFixture(idPtr(3), history)

	mirror.Transition(txContext(), 7, 42, constants.PhasePrOpen)

	assert.Equal(t, []model.IssueHistoryChange{
		{Field: model.IssueHistoryFieldState, OldValue: int64(3), NewValue: int64(99)},
	}, history.changes)
}

func TestTransition_RecordsNullAsTheOldStateOfAnUnsetIssue(t *testing.T) {
	history := &fakeHistoryRecorder{}
	mirror, _, _ := newHistoryFixture(nil, history)

	mirror.Transition(txContext(), 7, 42, constants.PhasePrOpen)

	require.Len(t, history.changes, 1)
	assert.Nil(t, history.changes[0].OldValue)
}

func TestTransition_SameStateRecordsNoHistory(t *testing.T) {
	history := &fakeHistoryRecorder{}
	mirror, store, _ := newHistoryFixture(idPtr(99), history)

	mirror.Transition(txContext(), 7, 42, constants.PhasePrOpen)

	require.NotNil(t, store.writtenState)
	assert.Empty(t, history.changes)
}

func TestTransition_HistoryFailureDoesNotBroadcast(t *testing.T) {
	mirror, _, notifier := newHistoryFixture(idPtr(3), &fakeHistoryRecorder{err: errors.New("history failed")})

	mirror.Transition(txContext(), 7, 42, constants.PhasePrOpen)

	assert.Empty(t, notifier.Notices(), "a state change whose history failed is rolled back and must not be announced")
}
