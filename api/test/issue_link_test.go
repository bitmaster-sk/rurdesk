package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func createLinkIssue(t *testing.T, app *issue.Application, token string, idProject int64, title string) int64 {
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"x"}`, title), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created.IdIssuePublic
}

func searchIssueLinks(t *testing.T, app *issue.Application, token, query string) []model.IssueLinkOption {
	res := Request(t, app, "GET", "/api/private/issue-link/search?"+query, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var options []model.IssueLinkOption
	require.Nil(t, json.NewDecoder(res.Body).Decode(&options))
	return options
}

func loadIssueLinkInfo(t *testing.T, app *issue.Application, token string, refs []model.IssueLinkRef) model.IssueLinkInfoRes {
	body, _ := json.Marshal(model.IssueLinkInfoReq{Refs: refs})
	res := Request(t, app, "POST", "/api/private/issue-link/info", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var info model.IssueLinkInfoRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&info))
	return info
}

func TestIssueLinkSearch_OnlyVisibleProjects(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	visible := createProject(t, app, token, "link-search-visible")
	hidden := createProject(t, app, token, "link-search-hidden")
	createLinkIssue(t, app, token, visible, "linksearch visible task")
	createLinkIssue(t, app, token, hidden, "linksearch hidden task")
	viewer := registerViewer(t, app, token, visible, "link-search-viewer@rurdesk.test")

	options := searchIssueLinks(t, app, viewer, "q=linksearch")

	require.Len(t, options, 1)
	require.Equal(t, visible, options[0].IdProject)
	require.Equal(t, "link-search-visible", options[0].ProjectName)
	require.Equal(t, "linksearch visible task", options[0].Title)
}

func TestIssueLinkSearch_CurrentProjectFirst(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	first := createProject(t, app, token, "link-order-first")
	other := createProject(t, app, token, "link-order-other")
	createLinkIssue(t, app, token, first, "linkorder older in current")
	createLinkIssue(t, app, token, other, "linkorder newer elsewhere")

	options := searchIssueLinks(t, app, token, fmt.Sprintf("q=linkorder&idProject=%d", first))

	require.Len(t, options, 2)
	require.Equal(t, first, options[0].IdProject)
	require.Equal(t, other, options[1].IdProject)
}

func TestIssueLinkSearch_FiltersByNumberAndTitle(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "link-filter")
	viewer := registerViewer(t, app, token, idProject, "link-filter-viewer@rurdesk.test")
	var ids []int64
	for i := 0; i < 12; i++ {
		ids = append(ids, createLinkIssue(t, app, token, idProject, fmt.Sprintf("Filter task %d", i)))
	}
	createLinkIssue(t, app, token, idProject, "Fix 100% of the login_flow")

	byNumber := searchIssueLinks(t, app, viewer, "q="+url.QueryEscape(fmt.Sprintf("#%d", ids[10])))
	require.NotEmpty(t, byNumber)
	require.Equal(t, ids[10], byNumber[0].IdIssuePublic)

	byTitle := searchIssueLinks(t, app, viewer, "q=login_flow")
	require.Len(t, byTitle, 1)
	require.Equal(t, "Fix 100% of the login_flow", byTitle[0].Title)

	require.Len(t, searchIssueLinks(t, app, viewer, "q="+url.QueryEscape("_")), 1)
	require.Len(t, searchIssueLinks(t, app, viewer, "q="+url.QueryEscape("%")), 1)

	newest := searchIssueLinks(t, app, viewer, "")
	require.Len(t, newest, 8)
	require.Equal(t, "Fix 100% of the login_flow", newest[0].Title)
}

func TestIssueLinkInfo_ReturnsVisibleIssuesWithCatalogs(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	visible := createProject(t, app, token, "link-info-visible")
	hidden := createProject(t, app, token, "link-info-hidden")
	me := currentUser(t, app, token)
	final, _ := finalAndNonFinalState(t, app, token, visible)
	closed := createAssignedIssue(t, app, token, visible, final.IdState, me.IdUser, "closed task")
	hiddenIssue := createLinkIssue(t, app, token, hidden, "hidden task")
	viewer := registerViewer(t, app, token, visible, "link-info-viewer@rurdesk.test")

	info := loadIssueLinkInfo(t, app, viewer, []model.IssueLinkRef{
		{IdProject: visible, IdIssuePublic: closed},
		{IdProject: visible, IdIssuePublic: closed},
		{IdProject: visible, IdIssuePublic: 999999},
		{IdProject: hidden, IdIssuePublic: hiddenIssue},
	})

	require.Len(t, info.Issues, 1)
	require.Equal(t, closed, info.Issues[0].IdIssuePublic)
	require.Equal(t, "closed task", info.Issues[0].Title)
	require.Equal(t, final.IdState, *info.Issues[0].IdState)
	require.Equal(t, me.IdUser, *info.Issues[0].AssignedTo)

	require.Equal(t, []model.IssueLinkProject{{IdProject: visible, Name: "link-info-visible"}}, info.Projects)

	statesByID := map[int64]*model.State{}
	for _, state := range info.States {
		require.Equal(t, visible, state.IdProject)
		statesByID[state.IdState] = state
	}
	require.True(t, statesByID[final.IdState].Final)
	require.Greater(t, len(info.States), 1, "unused states of the project must be listed too")

	require.NotEmpty(t, info.Severities)
	for _, severity := range info.Severities {
		require.Equal(t, visible, severity.IdProject)
	}

	require.NotEmpty(t, info.IssueTypes)
	for _, issueType := range info.IssueTypes {
		require.Equal(t, visible, issueType.IdProject)
	}

	var idsUser []int64
	for _, user := range info.Users {
		idsUser = append(idsUser, user.IdUser)
	}
	require.Contains(t, idsUser, me.IdUser)
}

func TestIssueLinkInfo_EmptyWhenNothingVisible(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	hidden := createProject(t, app, token, "link-info-nothing-hidden")
	visible := createProject(t, app, token, "link-info-nothing-visible")
	hiddenIssue := createLinkIssue(t, app, token, hidden, "hidden")
	viewer := registerViewer(t, app, token, visible, "link-info-nothing@rurdesk.test")

	info := loadIssueLinkInfo(t, app, viewer, []model.IssueLinkRef{{IdProject: hidden, IdIssuePublic: hiddenIssue}})

	require.Empty(t, info.Issues)
	require.Empty(t, info.Projects)
	require.Empty(t, info.States)
	require.Empty(t, info.Severities)
	require.Empty(t, info.IssueTypes)
	require.Empty(t, info.Users)
}

func TestIssueLinkInfo_RejectsTooManyRefs(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	refs := make([]model.IssueLinkRef, 101)
	for i := range refs {
		refs[i] = model.IssueLinkRef{IdProject: 1, IdIssuePublic: int64(i + 1)}
	}
	body, _ := json.Marshal(model.IssueLinkInfoReq{Refs: refs})

	res := Request(t, app, "POST", "/api/private/issue-link/info", string(body), token)

	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}
