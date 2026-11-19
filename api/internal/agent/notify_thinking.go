package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
)

type ThinkingNotifier struct {
	notifier notify.INotifier
	members  projectMemberLoader
}

func NewThinkingNotifier(notifier notify.INotifier, members projectMemberLoader) *ThinkingNotifier {
	return &ThinkingNotifier{notifier: notifier, members: members}
}

// BroadcastToProject sends the thinking notice to every member of the project.
func (n *ThinkingNotifier) BroadcastToProject(ctx context.Context, idProject int64, notice *model.AgentThinkingNotice) {
	if notice == nil {
		return
	}
	members, err := n.members.LoadProjectsMembers(ctx, []int64{idProject})
	// Unresolved membership drops the notice: a fallback to all sessions would
	// leak one project's thinking to everyone.
	if err != nil || len(members) == 0 {
		return
	}
	idsUser := make([]int64, len(members))
	for index, member := range members {
		idsUser[index] = member.IdUser
	}
	n.notifier.Send(&notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectAgentThinking,
		Action:  notify.ActionCreate,
		Payload: notice,
	})
}

// BroadcastToUser sends the thinking notice to that user alone, as a chat turn's goes to the
// conversation owner; without a user nothing is sent.
func (n *ThinkingNotifier) BroadcastToUser(_ context.Context, idUser int64, notice *model.AgentThinkingNotice) {
	if notice == nil || idUser == 0 {
		return
	}
	n.notifier.Send(&notify.Notice{
		IdsUser: []int64{idUser},
		Subject: notify.SubjectAgentThinking,
		Action:  notify.ActionCreate,
		Payload: notice,
	})
}
