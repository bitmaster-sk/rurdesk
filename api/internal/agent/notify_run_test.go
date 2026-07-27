package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIssueLoader struct {
	issue *model.Issue
	err   error
}

func (f fakeIssueLoader) LoadIssue(_ context.Context, _ *repository.LoadIssueFilter) (*model.Issue, error) {
	return f.issue, f.err
}

type fakeMemberLoader struct {
	members []*model.User
	err     error
}

func (f fakeMemberLoader) LoadProjectsMembers(_ context.Context, _ []int64) ([]*model.User, error) {
	return f.members, f.err
}

// newTestNotifier returns a Notifier with a buffered Send channel but no
// background listen loop, so tests can inspect the emitted notice directly
// instead of it being consumed by broadcast().
func newTestNotifier() *notify.Notifier {
	return &notify.Notifier{Send: make(chan *notify.Notice, 1)}
}

func TestBroadcastIssueUpdate_ScopesToProjectMembers(t *testing.T) {
	issue := &model.Issue{IdIssue: 42, IdProject: 7}
	n := newTestNotifier()

	BroadcastIssueUpdate(
		context.Background(),
		n,
		fakeIssueLoader{issue: issue},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}, {IdUser: 20}}},
		42,
	)

	require.Len(t, n.Send, 1, "expected exactly one notice to be emitted")
	notice := <-n.Send
	assert.Equal(t, []int64{10, 20}, notice.IdsUser, "notice must target project members only")
	assert.Equal(t, notify.SubjectIssue, notice.Subject)
	assert.Equal(t, notify.ActionUpdate, notice.Action)
	assert.Same(t, issue, notice.Payload, "payload must be the loaded issue")
}

func TestBroadcastIssueUpdate_NoMembersSkipsToAvoidGlobalBroadcast(t *testing.T) {
	n := newTestNotifier()

	BroadcastIssueUpdate(
		context.Background(),
		n,
		fakeIssueLoader{issue: &model.Issue{IdIssue: 42, IdProject: 7}},
		fakeMemberLoader{members: nil},
		42,
	)

	// An empty IdsUser would fall through to the notifier's IdUser==0 global
	// fan-out, so the function must emit nothing.
	assert.Empty(t, n.Send, "no notice may be sent when membership is unresolved")
}

func TestBroadcastIssueUpdate_MemberLoadErrorSkips(t *testing.T) {
	n := newTestNotifier()

	BroadcastIssueUpdate(
		context.Background(),
		n,
		fakeIssueLoader{issue: &model.Issue{IdIssue: 42, IdProject: 7}},
		fakeMemberLoader{err: errors.New("db down")},
		42,
	)

	assert.Empty(t, n.Send, "no notice may be sent when member load fails")
}

func TestBroadcastIssueUpdate_IssueLoadFailureSkips(t *testing.T) {
	n := newTestNotifier()

	BroadcastIssueUpdate(
		context.Background(),
		n,
		fakeIssueLoader{issue: nil},
		fakeMemberLoader{members: []*model.User{{IdUser: 10}}},
		42,
	)

	assert.Empty(t, n.Send, "no notice may be sent when the issue cannot be loaded")
}
