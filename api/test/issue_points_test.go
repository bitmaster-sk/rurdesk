package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// A full-replace edit that changes only the title must NOT wipe points (which
// rides the dto) or id_sprint (which the edit path never touches). This pins
// both the six-SELECT coverage (LoadIssue returns points) and the edit path.
func TestEditIssue_TitleOnlyPreservesPointsAndSprint(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-points")
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)

	issuePublic := createIssueWithPoints(t, app, token, idProject, states[0].IdState, 8)
	sprint := createSprint(t, app, token, idProject, `{}`)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, issuePublic, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)

	loaded := loadIssue(t, app, token, idProject, issuePublic)
	require.NotNil(t, loaded.Points)
	require.Equal(t, 8, *loaded.Points)

	// Mirror the detail form: resend the whole dto, changing only the title.
	editBody, _ := json.Marshal(model.EditIssueReq{
		IdProject:     idProject,
		IdIssuePublic: issuePublic,
		IdState:       model.NewOptionalPtr(loaded.IdState),
		IdSeverity:    model.NewOptionalPtr(loaded.IdSeverity),
		Title:         model.NewOptional("renamed"),
		Description:   model.NewOptional(loaded.Description),
		AssignedTo:    model.NewOptionalPtr(loaded.AssignedTo),
		Estimated:     model.NewOptional(loaded.Estimated),
		Points:        model.NewOptionalPtr(loaded.Points),
		ScheduledAt:   model.NewOptionalPtr(loaded.ScheduledAt),
	})
	editRes := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, issuePublic), string(editBody), token)
	require.Equal(t, http.StatusOK, editRes.StatusCode)

	after := loadIssue(t, app, token, idProject, issuePublic)
	require.Equal(t, "renamed", after.Title)
	require.NotNil(t, after.Points, "points must survive a title-only edit")
	require.Equal(t, 8, *after.Points)
	require.NotNil(t, after.IdSprint, "id_sprint must survive a title-only edit")
	require.Equal(t, sprint.IdSprint, *after.IdSprint)
}
