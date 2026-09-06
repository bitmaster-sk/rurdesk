package agent

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTaskAgents struct {
	idUserAgent int64
	err         error
}

func (f *fakeTaskAgents) LoadAgentForTask(ctx context.Context, idTask int64) (int64, error) {
	return f.idUserAgent, f.err
}

// The gateway callbacks accept any authenticated user, so this match is the
// whole authorization boundary.
func TestTaskService_IsAgentAssignedToTask(t *testing.T) {
	svc := NewTaskService(&fakeTaskAgents{idUserAgent: 7})

	isAgent, err := svc.IsAgentAssignedToTask(context.Background(), 7, 1)
	require.NoError(t, err)
	assert.True(t, isAgent)

	isAgent, err = svc.IsAgentAssignedToTask(context.Background(), 8, 1)
	require.NoError(t, err)
	assert.False(t, isAgent, "a user who is not the task's agent must be refused")

	isAgent, err = svc.IsAgentAssignedToTask(context.Background(), 0, 1)
	require.NoError(t, err)
	assert.False(t, isAgent, "an unresolved caller must be refused, never defaulted in")
}

// A missing task must stay distinguishable from a refusal, or a typo in a task
// id reads as revoked access.
func TestTaskService_IsAgentAssignedToTaskSurfacesAMissingTask(t *testing.T) {
	svc := NewTaskService(&fakeTaskAgents{err: repository.ErrTaskNotFound})

	isAgent, err := svc.IsAgentAssignedToTask(context.Background(), 7, 1)

	assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	assert.False(t, isAgent)
}
