package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
)

type ThinkingNotifier struct {
	notifier *notify.Notifier
	members  projectMemberLoader
}

func NewThinkingNotifier(notifier *notify.Notifier, members projectMemberLoader) *ThinkingNotifier {
	return &ThinkingNotifier{notifier: notifier, members: members}
}

// Broadcast sends the thinking notice to every member of the project.
func (n *ThinkingNotifier) Broadcast(ctx context.Context, idProject int64, notice *model.AgentThinkingNotice) {
	if n.notifier == nil || notice == nil {
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
	n.notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectAgentThinking,
		Action:  notify.ActionCreate,
		Payload: notice,
	}
}
