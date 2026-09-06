package agent

import (
	"context"
)

type taskAgentLoader interface {
	LoadAgentForTask(ctx context.Context, idTask int64) (int64, error)
}

type TaskService struct {
	tasks taskAgentLoader
}

func NewTaskService(tasks taskAgentLoader) *TaskService {
	return &TaskService{tasks: tasks}
}

// IsAgentAssignedToTask reports whether the user is the agent the task is
// assigned to; it says nothing about the task's status. A missing task comes
// back as repository.ErrTaskNotFound, not as a false.
//
// The gateway callbacks sit in the ordinary authenticated group, which accepts
// any user JWT — the API key is not a distinct principal — so this match is the
// whole authorization boundary.
func (s *TaskService) IsAgentAssignedToTask(ctx context.Context, idUser, idTask int64) (bool, error) {
	idUserAgent, err := s.tasks.LoadAgentForTask(ctx, idTask)
	if err != nil {
		return false, err
	}
	return idUser == idUserAgent, nil
}
