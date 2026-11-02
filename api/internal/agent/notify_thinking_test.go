package agent

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify/notifytest"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingMemberLoader struct {
	calls   int
	members []*model.User
}

func (l *countingMemberLoader) LoadProjectsMembers(_ context.Context, _ []int64) ([]*model.User, error) {
	l.calls++
	return l.members, nil
}

var _ projectMemberLoader = (*countingMemberLoader)(nil)
var _ projectMemberLoader = (*repository.ProjectRepository)(nil)

func newTestThinkingNotifier(loader *countingMemberLoader) (*ThinkingNotifier, *notifytest.Recorder) {
	sent := &notifytest.Recorder{}
	return NewThinkingNotifier(sent, loader), sent
}

func TestThinkingNotifier_SendsToProjectMembers(t *testing.T) {
	loader := &countingMemberLoader{members: []*model.User{{IdUser: 1}, {IdUser: 2}}}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	require.Len(t, sent.Notices(), 1)
	notice := sent.Notices()[0]
	assert.Equal(t, []int64{1, 2}, notice.IdsUser)
	assert.Equal(t, notify.SubjectAgentThinking, notice.Subject)
}

func TestThinkingNotifier_ResolvesMembersPerBatch(t *testing.T) {
	loader := &countingMemberLoader{members: []*model.User{{IdUser: 1}, {IdUser: 9}}}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})
	first := sent.Drain()
	require.Len(t, first, 1)
	require.Equal(t, []int64{1, 9}, first[0].IdsUser)

	loader.members = []*model.User{{IdUser: 1}}
	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	second := sent.Drain()
	require.Len(t, second, 1)
	assert.Equal(t, []int64{1}, second[0].IdsUser)
	assert.Equal(t, 2, loader.calls)
}

func TestThinkingNotifier_SkipsWhenNoMembers(t *testing.T) {
	loader := &countingMemberLoader{}
	notifier, sent := newTestThinkingNotifier(loader)

	notifier.Broadcast(context.Background(), 7, &model.AgentThinkingNotice{IdRun: 3})

	assert.Empty(t, sent.Notices())
}
