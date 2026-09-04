package agent

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingMemberLoader struct {
	calls   int
	members []*model.User
}

func (l *countingMemberLoader) LoadProjectsMembers(ctx context.Context, idsProject []int64) ([]*model.User, error) {
	l.calls++
	return l.members, nil
}

var _ projectMemberLoader = (*countingMemberLoader)(nil)
var _ projectMemberLoader = (*repository.ProjectRepository)(nil)

func newTestThinkingNotifier(loader *countingMemberLoader) (*ThinkingNotifier, chan *notify.Notice) {
	sent := make(chan *notify.Notice, 4)
	return NewThinkingNotifier(&notify.Notifier{Send: sent}, loader), sent
}

func TestThinkingNotifier_SendsToProjectMembers(t *testing.T) {
	loader := &countingMemberLoader{members: []*model.User{{IdUser: 1}, {IdUser: 2}}}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	notice := <-sent
	assert.Equal(t, []int64{1, 2}, notice.IdsUser)
	assert.Equal(t, notify.SubjectAgentThinking, notice.Subject)
}

func TestThinkingNotifier_ResolvesMembersPerBatch(t *testing.T) {
	loader := &countingMemberLoader{members: []*model.User{{IdUser: 1}, {IdUser: 9}}}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})
	require.Equal(t, []int64{1, 9}, (<-sent).IdsUser)

	loader.members = []*model.User{{IdUser: 1}}
	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	assert.Equal(t, []int64{1}, (<-sent).IdsUser)
	assert.Equal(t, 2, loader.calls)
}

func TestThinkingNotifier_SkipsWhenNoMembers(t *testing.T) {
	loader := &countingMemberLoader{}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	assert.Empty(t, sent)
}
