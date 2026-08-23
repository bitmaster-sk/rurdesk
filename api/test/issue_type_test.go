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

func loadIssueTypes(t *testing.T, app *issue.Application, token string, idProject int64) []model.IssueType {
	res := Request(t, app, "GET", "/api/private/issue-type", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.IssueType
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	mine := make([]model.IssueType, 0, len(all))
	for _, it := range all {
		if it.IdProject == idProject {
			mine = append(mine, it)
		}
	}
	return mine
}

func createIssueType(t *testing.T, app *issue.Application, token string, idProject int64, name string) model.IssueType {
	body, _ := json.Marshal(model.CreateIssueTypeReq{IdProject: idProject, Name: name})
	res := Request(t, app, "POST", "/api/private/issue-type", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.IssueType
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created
}

func createIssueWithType(t *testing.T, app *issue.Application, token string, idProject int64, title string, idIssueType *int64) model.Issue {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		Title:       title,
		Description: "x",
		IdIssueType: idIssueType,
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created
}

func TestNewProject_SeedsThreeIssueTypes(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-seed")

	types := loadIssueTypes(t, app, token, idProject)
	require.Len(t, types, 3)
	require.Equal(t, []string{"Bug", "Feature", "Task"}, []string{types[0].Name, types[1].Name, types[2].Name})
	require.Equal(t, []int{1, 2, 3}, []int{types[0].OrderRank, types[1].OrderRank, types[2].OrderRank})
	for _, it := range types {
		require.False(t, it.Protected)
	}
}

func TestNewProject_DefaultIssueTypeIsUnset(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-default-null")

	res := Request(t, app, "GET", "/api/private/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var project model.Project
	require.Nil(t, json.NewDecoder(res.Body).Decode(&project))
	require.Nil(t, project.IdIssueTypeDefault)
}

func TestCreateIssueType_AppendsAtEnd(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-append")

	created := createIssueType(t, app, token, idProject, "Spike")
	require.Equal(t, "Spike", created.Name)
	require.Equal(t, 4, created.OrderRank)
	require.False(t, created.Protected)
}

func TestEditIssueType_RenamesAndReorders(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-edit")
	types := loadIssueTypes(t, app, token, idProject)
	last := types[2]

	body, _ := json.Marshal(model.EditIssueTypeReq{
		IdProject: idProject,
		Name:      "Chore",
		OrderRank: 1,
	})
	res := Request(t, app, "PATCH", "/api/private/issue-type/"+itoa(last.IdIssueType), string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	after := loadIssueTypes(t, app, token, idProject)
	require.Len(t, after, 3)
	require.Equal(t, "Chore", after[0].Name)
	require.Equal(t, last.IdIssueType, after[0].IdIssueType)
	require.Equal(t, []int{1, 2, 3}, []int{after[0].OrderRank, after[1].OrderRank, after[2].OrderRank})
}

func TestDeleteIssueType_UnusedNeedsNoIntent(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-del-unused")
	spike := createIssueType(t, app, token, idProject, "Spike")

	res := Request(t, app, "DELETE", "/api/private/issue-type/"+itoa(spike.IdIssueType)+"/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Len(t, loadIssueTypes(t, app, token, idProject), 3)
}

func TestDeleteIssueType_InUseWithoutIntentIsRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-del-inuse")
	types := loadIssueTypes(t, app, token, idProject)
	bug := types[0]
	createIssueWithType(t, app, token, idProject, "uses bug", &bug.IdIssueType)

	res := Request(t, app, "DELETE", "/api/private/issue-type/"+itoa(bug.IdIssueType)+"/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
	require.Len(t, loadIssueTypes(t, app, token, idProject), 3)
}

func TestDeleteIssueType_MigratesIssuesToTarget(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-del-migrate")
	types := loadIssueTypes(t, app, token, idProject)
	bug, feature := types[0], types[1]
	created := createIssueWithType(t, app, token, idProject, "migrate me", &bug.IdIssueType)

	url := "/api/private/issue-type/" + itoa(bug.IdIssueType) + "/project/" + itoa(idProject) +
		"?migrateTo=" + itoa(feature.IdIssueType)
	res := Request(t, app, "DELETE", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/issue/"+itoa(created.IdIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var reloaded model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&reloaded))
	require.NotNil(t, reloaded.IdIssueType)
	require.Equal(t, feature.IdIssueType, *reloaded.IdIssueType)
}

func TestDeleteIssueType_UnassignClearsIssuesAndProjectDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-del-unassign")
	types := loadIssueTypes(t, app, token, idProject)
	bug := types[0]
	created := createIssueWithType(t, app, token, idProject, "unassign me", &bug.IdIssueType)

	body, _ := json.Marshal(model.EditProjectReq{
		IdProject:          idProject,
		Name:               "it-del-unassign",
		IdIssueTypeDefault: &bug.IdIssueType,
	})
	res := Request(t, app, "PATCH", "/api/private/project", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	url := "/api/private/issue-type/" + itoa(bug.IdIssueType) + "/project/" + itoa(idProject) + "?migrateTo=null"
	res = Request(t, app, "DELETE", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/issue/"+itoa(created.IdIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var reloaded model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&reloaded))
	require.Nil(t, reloaded.IdIssueType)

	res = Request(t, app, "GET", "/api/private/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var project model.Project
	require.Nil(t, json.NewDecoder(res.Body).Decode(&project))
	require.Nil(t, project.IdIssueTypeDefault)
}

func TestIssueTypeUsage_ReportsIssuesAndDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-usage")
	types := loadIssueTypes(t, app, token, idProject)
	bug := types[0]
	createIssueWithType(t, app, token, idProject, "u1", &bug.IdIssueType)
	createIssueWithType(t, app, token, idProject, "u2", &bug.IdIssueType)

	body, _ := json.Marshal(model.EditProjectReq{
		IdProject:          idProject,
		Name:               "it-usage",
		IdIssueTypeDefault: &bug.IdIssueType,
	})
	res := Request(t, app, "PATCH", "/api/private/project", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "GET", "/api/private/issue-type/"+itoa(bug.IdIssueType)+"/project/"+itoa(idProject)+"/usage", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var usage model.IssueTypeUsage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&usage))
	require.Equal(t, 2, usage.Issues)
	require.True(t, usage.IsProjectDefault)
}

func TestDeleteIssueType_MigrateToForeignProjectRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "it-foreign-A")
	projectB := createProject(t, app, token, "it-foreign-B")
	typesA := loadIssueTypes(t, app, token, projectA)
	typesB := loadIssueTypes(t, app, token, projectB)
	createIssueWithType(t, app, token, projectA, "a1", &typesA[0].IdIssueType)

	url := "/api/private/issue-type/" + itoa(typesA[0].IdIssueType) + "/project/" + itoa(projectA) +
		"?migrateTo=" + itoa(typesB[0].IdIssueType)
	res := Request(t, app, "DELETE", url, "", token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Len(t, loadIssueTypes(t, app, token, projectA), 3)
}

func TestCreateIssue_ForeignIssueType_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "it-scope-A")
	projectB := createProject(t, app, token, "it-scope-B")
	foreign := loadIssueTypes(t, app, token, projectB)[0]

	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   projectA,
		Title:       "cross-project type",
		Description: "x",
		IdIssueType: &foreign.IdIssueType,
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(projectA)+"/issue", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestListIssues_FilterByIssueType(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-filter")
	types := loadIssueTypes(t, app, token, idProject)
	bug, feature := types[0], types[1]
	createIssueWithType(t, app, token, idProject, "a bug", &bug.IdIssueType)
	createIssueWithType(t, app, token, idProject, "a feature", &feature.IdIssueType)
	createIssueWithType(t, app, token, idProject, "no type", nil)

	url := "/api/private/project/" + itoa(idProject) + "/issue?idsIssueType=" + itoa(bug.IdIssueType)
	filtered := loadIssuesPage(t, app, token, url)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, "a bug", filtered.Items[0].Title)

	withUnset := loadIssuesPage(t, app, token, url+"&issueTypeUnset=true")
	require.Len(t, withUnset.Items, 2)
}

func TestIssueTypeEndpoints_ForbiddenForNonMember(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "it-acl")
	existing := loadIssueTypes(t, app, owner, idProject)[0]

	outsider := createUserAsAdmin(t, app, owner,
		`{"name":"it-out","email":"it-outsider@test.sk","password":"kreslo"}`)

	createBody, _ := json.Marshal(model.CreateIssueTypeReq{IdProject: idProject, Name: "Sneaky"})
	editBody, _ := json.Marshal(model.EditIssueTypeReq{
		IdProject: idProject,
		Name:      "Renamed",
		OrderRank: 1,
	})

	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"create", "POST", "/api/private/issue-type", string(createBody)},
		{"edit", "PATCH", "/api/private/issue-type/" + itoa(existing.IdIssueType), string(editBody)},
		{
			"usage",
			"GET",
			"/api/private/issue-type/" + itoa(existing.IdIssueType) + "/project/" + itoa(idProject) + "/usage",
			"",
		},
		{
			"delete",
			"DELETE",
			"/api/private/issue-type/" + itoa(existing.IdIssueType) + "/project/" + itoa(idProject),
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Request(t, app, tc.method, tc.url, tc.body, outsider)
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})
	}

	after := loadIssueTypes(t, app, owner, idProject)
	require.Len(t, after, 3)
	require.Equal(t, []string{"Bug", "Feature", "Task"}, []string{after[0].Name, after[1].Name, after[2].Name})
}

func TestIssueTypeList_HidesOtherProjectsFromNonMember(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "it-acl-list")

	outsider := createUserAsAdmin(t, app, owner,
		`{"name":"it-out2","email":"it-outsider2@test.sk","password":"kreslo"}`)

	res := Request(t, app, "GET", "/api/private/issue-type", "", outsider)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var visible []model.IssueType
	require.Nil(t, json.NewDecoder(res.Body).Decode(&visible))
	for _, it := range visible {
		require.NotEqual(t, idProject, it.IdProject)
	}
}

func TestSavedView_RoundTripsIssueTypeFilter(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-saved-view")
	types := loadIssueTypes(t, app, token, idProject)

	config := fmt.Sprintf(
		`{"v":1,"idsIssueType":[%d],"issueTypeUnset":true,"orderColumn":"issueType","orderDirection":"asc"}`,
		types[0].IdIssueType,
	)
	body, _ := json.Marshal(model.CreateSavedViewReq{
		Name:     "Bugs only",
		ViewType: "table",
		Config:   json.RawMessage(config),
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/saved-view", string(body), token)
	require.Equal(t, http.StatusCreated, res.StatusCode)

	res = Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/saved-view", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var views []model.SavedView
	require.Nil(t, json.NewDecoder(res.Body).Decode(&views))
	require.Len(t, views, 1)

	var reloaded struct {
		IdsIssueType   []int64 `json:"idsIssueType"`
		IssueTypeUnset bool    `json:"issueTypeUnset"`
		OrderColumn    string  `json:"orderColumn"`
	}
	require.Nil(t, json.Unmarshal(views[0].Config, &reloaded))
	require.Equal(t, []int64{types[0].IdIssueType}, reloaded.IdsIssueType)
	require.True(t, reloaded.IssueTypeUnset)
	require.Equal(t, "issueType", reloaded.OrderColumn)
}

func TestListIssues_SortsByIssueTypeOnBothPaths(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "it-sort")
	types := loadIssueTypes(t, app, token, idProject)
	createIssueWithType(t, app, token, idProject, "third", &types[2].IdIssueType)
	createIssueWithType(t, app, token, idProject, "first", &types[0].IdIssueType)

	base := "/api/private/project/" + itoa(idProject) +
		"/issue?orderColumn=issueType&orderDirection=asc"

	unpaged := loadIssuesPage(t, app, token, base)
	require.Len(t, unpaged.Items, 2)
	require.Equal(t, "first", unpaged.Items[0].Title)

	paged := loadIssuesPage(t, app, token, base+"&limit=10")
	require.Len(t, paged.Items, 2)
	require.Equal(t, "first", paged.Items[0].Title)

	body, _ := json.Marshal(model.CreateSavedViewReq{
		Name:     "By type",
		ViewType: "table",
		Config:   json.RawMessage(`{"v":1,"orderColumn":"issueType","orderDirection":"asc"}`),
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/saved-view", string(body), token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
}

func TestUpdateProject_ForeignOrUnknownIssueTypeDefaultRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "it-default-A")
	projectB := createProject(t, app, token, "it-default-B")
	foreign := loadIssueTypes(t, app, token, projectB)[0]
	own := loadIssueTypes(t, app, token, projectA)[0]
	unknown := int64(9_000_000)

	patch := func(idIssueTypeDefault *int64) int {
		body, _ := json.Marshal(model.EditProjectReq{
			IdProject:          projectA,
			Name:               "it-default-A",
			IdIssueTypeDefault: idIssueTypeDefault,
		})
		return Request(t, app, "PATCH", "/api/private/project", string(body), token).StatusCode
	}

	require.Equal(t, http.StatusBadRequest, patch(&foreign.IdIssueType))
	require.Equal(t, http.StatusBadRequest, patch(&unknown))
	require.Equal(t, http.StatusOK, patch(&own.IdIssueType))
	require.Equal(t, http.StatusOK, patch(nil))

	res := Request(t, app, "GET", "/api/private/project/"+itoa(projectA), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var project model.Project
	require.Nil(t, json.NewDecoder(res.Body).Decode(&project))
	require.Nil(t, project.IdIssueTypeDefault)
}
