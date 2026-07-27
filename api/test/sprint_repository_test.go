package test

import (
	"context"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestSprintRepository_InsertLoadAndMaxSeq(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-repo-p1")

	repo := injector.GetSprintRepository()
	ctx := context.Background()

	for _, name := range []string{"Sprint 3", "Sprint 12", "Kickoff"} {
		_, err := repo.Insert(ctx, &model.Sprint{
			IdProject: idProject,
			Name:      name,
			StartAt:   time.Now().UTC(),
			EndAt:     time.Now().UTC().Add(14 * 24 * time.Hour),
			State:     "planned",
		}, 1)
		require.Nil(t, err, "insert %s", name)
	}

	list, err := repo.LoadByProject(ctx, idProject)
	require.Nil(t, err)
	require.Len(t, list, 3)

	maxSeq, err := repo.MaxNameSeq(ctx, idProject)
	require.Nil(t, err)
	require.Equal(t, 12, maxSeq, "highest trailing integer among sprint names")
}

func TestSprintRepository_AssignIssueRejectsForeignProject(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "sprint-repo-pa")
	idProjectB := createProject(t, app, token, "sprint-repo-pb")

	statesA := loadProjectStates(t, app, token, idProjectA)
	require.NotEmpty(t, statesA)
	idIssuePublicA := createIssueInState(t, app, token, idProjectA, statesA[0].IdState)

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	sprintB, err := repo.Insert(ctx, &model.Sprint{
		IdProject: idProjectB,
		Name:      "Sprint 1",
		StartAt:   time.Now().UTC(),
		EndAt:     time.Now().UTC().Add(14 * 24 * time.Hour),
		State:     "planned",
	}, 1)
	require.Nil(t, err)

	// Issue in project A cannot join a sprint owned by project B.
	ok, err := repo.AssignIssue(ctx, idProjectA, idIssuePublicA, &sprintB.IdSprint, 1)
	require.Nil(t, err)
	require.False(t, ok, "cross-project sprint assignment must be rejected")

	// Sanity: clearing (nil sprint) on a real issue succeeds.
	ok, err = repo.AssignIssue(ctx, idProjectA, idIssuePublicA, nil, 1)
	require.Nil(t, err)
	require.True(t, ok)
}
