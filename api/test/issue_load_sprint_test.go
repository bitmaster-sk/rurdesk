package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestLoadIssues_FilterBySprint(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-filter")
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)

	inSprint := createIssueInState(t, app, token, idProject, states[0].IdState)
	_ = createIssueInState(t, app, token, idProject, states[0].IdState) // backlog issue, not assigned
	sprint := createSprint(t, app, token, idProject, `{}`)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, inSprint, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)

	res := Request(t, app, "GET",
		fmt.Sprintf("/api/private/project/%d/issue?limit=200&idSprint=%d", idProject, sprint.IdSprint), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page model.IssuesPageRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&page))

	require.Len(t, page.Items, 1, "only the in-sprint issue matches the filter")
	require.Equal(t, inSprint, page.Items[0].IdIssuePublic)
	require.NotNil(t, page.Items[0].IdSprint)
	require.Equal(t, sprint.IdSprint, *page.Items[0].IdSprint)
}

func TestLoadIssues_BacklogUnset(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-backlog")
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)

	inSprint := createIssueInState(t, app, token, idProject, states[0].IdState)
	backlog := createIssueInState(t, app, token, idProject, states[0].IdState)
	sprint := createSprint(t, app, token, idProject, `{}`)
	assignSprint(t, app, token, idProject, inSprint, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))

	res := Request(t, app, "GET",
		fmt.Sprintf("/api/private/project/%d/issue?limit=200&sprintUnset=true", idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page model.IssuesPageRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&page))

	require.Len(t, page.Items, 1, "Backlog shows only issues with no sprint")
	require.Equal(t, backlog, page.Items[0].IdIssuePublic)
	require.Nil(t, page.Items[0].IdSprint)
}

func TestLoadIssuesGrouped_BySprint(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-group")
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)

	inSprint := createIssueInState(t, app, token, idProject, states[0].IdState)
	backlog := createIssueInState(t, app, token, idProject, states[0].IdState)
	sprint := createSprint(t, app, token, idProject, `{}`)
	assignSprint(t, app, token, idProject, inSprint, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))

	res := Request(t, app, "GET",
		fmt.Sprintf("/api/private/project/%d/issue?limit=200&groupBy=state,sprint", idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body struct {
		Groups []struct {
			Key   map[string]any `json:"key"`
			Items []model.Issue  `json:"items"`
		} `json:"groups"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))

	var sprintGroupHasIssue, backlogGroupHasIssue bool
	for _, g := range body.Groups {
		idSprintKey := g.Key["idSprint"]
		for _, it := range g.Items {
			if it.IdIssuePublic == inSprint {
				require.Equal(t, float64(sprint.IdSprint), idSprintKey, "in-sprint issue grouped under its sprint")
				sprintGroupHasIssue = true
			}
			if it.IdIssuePublic == backlog {
				require.Nil(t, idSprintKey, "backlog issue grouped under the null-sprint group")
				backlogGroupHasIssue = true
			}
		}
	}
	require.True(t, sprintGroupHasIssue, "assigned issue must appear in a sprint-keyed group")
	require.True(t, backlogGroupHasIssue, "unassigned issue must appear in the null-sprint group")
}
