package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// Both wire forms must filter: ?idsState=1&idsState=2 (MCP) and ?idsState=1,2
// (Angular client). Unfixed, a comma value failed ParseInt as a whole and the
// list was dropped; since issue_repository gates on len() > 0 that removed the
// predicate instead of erroring, so the default table showed done tasks.
//
// Tests MUST use 2+ ids — a single id parses fine even unfixed.

// issueFilterURL builds the flat-list query with the *Unset flags the controller
// expects, leaving the state filter to the caller so each test can pick a wire form.
func issueFilterURL(idProject int64, stateQuery string) string {
	return fmt.Sprintf(
		"/api/private/project/%d/issue?%s&stateUnset=false&severityUnset=false&assignedToUnset=false",
		idProject, stateQuery,
	)
}

func loadIssuesPage(t *testing.T, app *issue.Application, token, url string) model.IssuesPageRes {
	res := Request(t, app, "GET", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page model.IssuesPageRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&page))
	return page
}

func publicIds(page model.IssuesPageRes) []int64 {
	ids := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.IdIssuePublic)
	}
	return ids
}

// setupThreeStates creates a project with one issue in each of its three default
// states and returns the project id, the states, and the three public issue ids.
func setupThreeStates(t *testing.T, app *issue.Application, token, name string) (int64, []model.State, []int64) {
	idProject := createProject(t, app, token, name)
	states := loadProjectStates(t, app, token, idProject)
	require.GreaterOrEqual(t, len(states), 3, "a fresh project seeds three default states")
	ids := []int64{
		createIssueInState(t, app, token, idProject, states[0].IdState),
		createIssueInState(t, app, token, idProject, states[1].IdState),
		createIssueInState(t, app, token, idProject, states[2].IdState),
	}
	return idProject, states, ids
}

func TestGetIssues_CommaJoinedStateFilterApplies(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject, states, ids := setupThreeStates(t, app, token, "ids-comma")

	page := loadIssuesPage(t, app, token, issueFilterURL(idProject,
		fmt.Sprintf("idsState=%d,%d", states[0].IdState, states[1].IdState)))

	require.Len(t, page.Items, 2,
		"unfixed code drops the whole comma-joined value and returns all three issues")
	require.ElementsMatch(t, []int64{ids[0], ids[1]}, publicIds(page))
}

func TestGetIssues_RepeatedStateParamsApply(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject, states, ids := setupThreeStates(t, app, token, "ids-repeated")

	page := loadIssuesPage(t, app, token, issueFilterURL(idProject,
		fmt.Sprintf("idsState=%d&idsState=%d", states[0].IdState, states[1].IdState)))

	require.Len(t, page.Items, 2, "the repeated form is the MCP path and must keep working")
	require.ElementsMatch(t, []int64{ids[0], ids[1]}, publicIds(page))
}

// Both forms in one request — no client does this today, but accepting both means
// the combination must not double-count or drop a value.
func TestGetIssues_MixedIdFormsApply(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject, states, ids := setupThreeStates(t, app, token, "ids-mixed")

	page := loadIssuesPage(t, app, token, issueFilterURL(idProject,
		fmt.Sprintf("idsState=%d,%d&idsState=%d",
			states[0].IdState, states[1].IdState, states[2].IdState)))

	require.Len(t, page.Items, 3)
	require.ElementsMatch(t, ids, publicIds(page))
}

// An unparseable element is skipped, not fatal: the request succeeds and the valid
// ids still filter.
func TestGetIssues_InvalidIdElementIsSkipped(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject, states, ids := setupThreeStates(t, app, token, "ids-invalid")

	page := loadIssuesPage(t, app, token, issueFilterURL(idProject,
		fmt.Sprintf("idsState=%d,abc", states[0].IdState)))

	require.Len(t, page.Items, 1)
	require.ElementsMatch(t, []int64{ids[0]}, publicIds(page))
}
