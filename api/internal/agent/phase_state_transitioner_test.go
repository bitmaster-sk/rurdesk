package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
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

func idPtr(id int64) *int64 { return &id }

func newMirrorFixture(
	mapping *model.WorkflowEventMapping,
	state *model.State,
) (*PhaseStateTransitioner, *fakeIssueStateStore, *notify.Notifier) {
	store := &fakeIssueStateStore{issue: &model.Issue{IdIssue: 42, IdProject: 7}}
	notifier := newTestNotifier()
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{mapping: mapping},
		store,
		fakeStateLoader{state: state},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
	)
	return mirror, store, notifier
}

func TestTransition_WritesStateAndBroadcasts(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(
		&model.WorkflowEventMapping{IdState: idPtr(99)},
		&model.State{IdState: 99, Name: "In QA"},
	)

	mirror.Transition(context.Background(), 7, 42, constants.PhasePrOpen)

	require.NotNil(t, store.writtenState, "the mapped state must be written to the issue")
	assert.Equal(t, int64(99), *store.writtenState)

	require.Len(t, notifier.Send, 1, "a state change must reach the project over the websocket")
	notice := <-notifier.Send
	assert.Equal(t, notify.SubjectIssue, notice.Subject)
	assert.Equal(t, []int64{10}, notice.IdsUser)
}

func TestTransition_NoMappingChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(nil, nil)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseQueued)

	assert.Nil(t, store.writtenState, "an unmapped event must leave the issue state alone")
	assert.Empty(t, notifier.Send, "an unmapped event must not broadcast")
}

func TestTransition_NullMappedStateChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(&model.WorkflowEventMapping{IdState: nil}, nil)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseDone)

	assert.Nil(t, store.writtenState)
	assert.Empty(t, notifier.Send)
}

func TestTransition_MissingStateChangesNothing(t *testing.T) {
	mirror, store, notifier := newMirrorFixture(&model.WorkflowEventMapping{IdState: idPtr(99)}, nil)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseDone)

	assert.Nil(t, store.writtenState, "a mapping pointing at a deleted state must not be applied")
	assert.Empty(t, notifier.Send)
}

func TestTransition_MappingLoadErrorChangesNothing(t *testing.T) {
	store := &fakeIssueStateStore{issue: &model.Issue{IdIssue: 42, IdProject: 7}}
	notifier := newTestNotifier()
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{err: errors.New("db down")},
		store,
		fakeStateLoader{state: &model.State{IdState: 99}},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
	)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseFailed)

	assert.Nil(t, store.writtenState)
	assert.Empty(t, notifier.Send)
}

func TestTransition_WriteFailureDoesNotBroadcast(t *testing.T) {
	store := &fakeIssueStateStore{
		issue:     &model.Issue{IdIssue: 42, IdProject: 7},
		updateErr: errors.New("write failed"),
	}
	notifier := newTestNotifier()
	mirror := NewPhaseStateTransitioner(
		&fakeMappingLoader{mapping: &model.WorkflowEventMapping{IdState: idPtr(99)}},
		store,
		fakeStateLoader{state: &model.State{IdState: 99, Name: "In QA"}},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		notifier,
	)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseDone)

	assert.Empty(t, notifier.Send, "a failed write must not announce a state it did not make")
}

func TestTransition_LooksUpTheEventItWasGiven(t *testing.T) {
	loader := &fakeMappingLoader{}
	mirror := NewPhaseStateTransitioner(
		loader,
		&fakeIssueStateStore{},
		fakeStateLoader{},
		fakeMemberLoader{},
		newTestNotifier(),
	)

	mirror.Transition(context.Background(), 7, 42, constants.PhaseAwaitingInput)

	assert.Equal(t, []string{constants.PhaseAwaitingInput}, loader.events)
}
