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

func createSavedView(t *testing.T, app *issue.Application, token string, idProject int64, body string) model.SavedView {
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject), body, token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var view model.SavedView
	require.Nil(t, json.NewDecoder(res.Body).Decode(&view))
	return view
}

func loadSavedViews(t *testing.T, app *issue.Application, token string, idProject int64) []model.SavedView {
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/saved-view", idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var list []model.SavedView
	require.Nil(t, json.NewDecoder(res.Body).Decode(&list))
	return list
}

func errorCode(t *testing.T, res *http.Response) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	return body.Code
}

// registerMemberInProject registers a fresh user, adds them to the project as
// member, and returns their token. member (not viewer) matters: CanCreateIssue
// is member+, CanReadProject is viewer+.
func registerMemberInProject(t *testing.T, app *issue.Application, ownerToken string, idProject int64, email string) string {
	Request(t, app, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"m","email":"%s","password":"kreslo"}`, email), ownerToken)
	loginRes := Request(t, app, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":"%s","password":"kreslo"}`, email), "")
	var tk struct{ Token string }
	require.Nil(t, json.NewDecoder(loginRes.Body).Decode(&tk))

	userRes := Request(t, app, "GET", "/api/private/user", "", tk.Token)
	var user model.User
	require.Nil(t, json.NewDecoder(userRes.Body).Decode(&user))

	Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, user.IdUser), ownerToken)
	return tk.Token
}

// registerOutsider registers a user and adds them to NO project, so every ACL check
// against someone else's project must fail for them.
func registerOutsider(t *testing.T, app *issue.Application, adminToken, email string) string {
	Request(t, app, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"o","email":"%s","password":"kreslo"}`, email), adminToken)
	loginRes := Request(t, app, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":"%s","password":"kreslo"}`, email), "")
	var tk struct{ Token string }
	require.Nil(t, json.NewDecoder(loginRes.Body).Decode(&tk))
	return tk.Token
}

func TestSavedViewApi_CrudRoundTrip(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-crud")

	view := createSavedView(t, app, token, idProject,
		`{"name":"My bugs","viewType":"table","config":{"v":1,"orderColumn":"updateAt","orderDirection":"desc"},"isShared":false}`)
	require.Equal(t, "My bugs", view.Name)
	require.Equal(t, "table", view.ViewType)
	require.False(t, view.IsShared)
	require.NotZero(t, view.IdSavedView)

	list := loadSavedViews(t, app, token, idProject)
	require.Len(t, list, 1)
	require.JSONEq(t, `{"v":1,"orderColumn":"updateAt","orderDirection":"desc"}`, string(list[0].Config))

	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Renamed","viewType":"kanban","config":{"v":1},"isShared":true}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var edited model.SavedView
	require.Nil(t, json.NewDecoder(res.Body).Decode(&edited))
	require.Equal(t, "Renamed", edited.Name)
	require.Equal(t, "kanban", edited.ViewType)
	require.True(t, edited.IsShared)

	res = Request(t, app, "DELETE", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView), "", token)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	require.Empty(t, loadSavedViews(t, app, token, idProject))
}

func TestSavedViewApi_PrivateViewInvisibleToOthers(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-private")
	createSavedView(t, app, owner, idProject,
		`{"name":"Mine","viewType":"table","config":{"v":1},"isShared":false}`)

	member := registerMemberInProject(t, app, owner, idProject, "sv-private-member@test.sk")
	require.Empty(t, loadSavedViews(t, app, member, idProject), "private views of others must not be listed")
}

func TestSavedViewApi_SharedViewVisibleButNotEditableByMember(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-shared")
	view := createSavedView(t, app, owner, idProject,
		`{"name":"Team view","viewType":"table","config":{"v":1},"isShared":true}`)

	member := registerMemberInProject(t, app, owner, idProject, "sv-shared-member@test.sk")
	require.Len(t, loadSavedViews(t, app, member, idProject), 1)

	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Hijacked","viewType":"table","config":{"v":1},"isShared":true}`, member)
	require.Equal(t, http.StatusForbidden, res.StatusCode, "member is neither creator nor project owner")

	res = Request(t, app, "DELETE", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView), "", member)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

// A member owns what they create, including inside someone else's project.
func TestSavedViewApi_MemberCanEditOwnView(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-own")
	member := registerMemberInProject(t, app, owner, idProject, "sv-own-member@test.sk")

	view := createSavedView(t, app, member, idProject,
		`{"name":"Mine","viewType":"table","config":{"v":1},"isShared":false}`)
	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Still mine","viewType":"table","config":{"v":1},"isShared":false}`, member)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

// The project owner is the escape hatch for a shared view whose author left.
func TestSavedViewApi_ProjectOwnerCanEditSomeoneElsesView(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-owner-override")
	member := registerMemberInProject(t, app, owner, idProject, "sv-override-member@test.sk")
	view := createSavedView(t, app, member, idProject,
		`{"name":"Member view","viewType":"table","config":{"v":1},"isShared":true}`)

	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Curated","viewType":"table","config":{"v":1},"isShared":true}`, owner)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestSavedViewApi_RejectsUnknownSortColumn(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-badsort")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Bad","viewType":"table","config":{"v":1,"orderColumn":"dropTables"},"isShared":false}`, token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_BAD_SORT", errorCode(t, res))
}

func TestSavedViewApi_RejectsOversizeConfig(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-oversize")

	junk := make([]byte, 9000)
	for i := range junk {
		junk[i] = 'a'
	}
	body := fmt.Sprintf(`{"name":"Big","viewType":"table","config":{"v":1,"junk":"%s"},"isShared":false}`, junk)
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject), body, token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_CONFIG_TOO_LARGE", errorCode(t, res))
}

// A JSON array parses but is not a config object — reject rather than store a
// shape the client can never read back.
func TestSavedViewApi_RejectsNonObjectConfig(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-nonobject")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Array","viewType":"table","config":[1,2,3],"isShared":false}`, token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_CONFIG_INVALID", errorCode(t, res))
}

// `null` and `[...]` are valid JSON that the client cannot read back — a stored one
// breaks the Views panel for everyone who can see the row, unrepairable from the UI.
func TestSavedViewApi_RejectsNullConfig(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-nullconfig")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Null","viewType":"table","config":null,"isShared":false}`, token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_CONFIG_INVALID", errorCode(t, res))
}

// The edit path validates too — without this, dropping the call from Edit keeps the
// suite green while letting a bad config in through the back door.
func TestSavedViewApi_EditValidatesConfig(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-editvalid")
	view := createSavedView(t, app, token, idProject,
		`{"name":"Fine","viewType":"table","config":{"v":1},"isShared":false}`)

	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Bad","viewType":"table","config":{"v":1,"orderColumn":"dropTables"},"isShared":false}`, token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_BAD_SORT", errorCode(t, res))

	junk := make([]byte, 9000)
	for i := range junk {
		junk[i] = 'a'
	}
	res = Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		fmt.Sprintf(`{"name":"Big","viewType":"table","config":{"v":1,"junk":"%s"},"isShared":false}`, junk), token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SAVED_VIEW_CONFIG_TOO_LARGE", errorCode(t, res))
}

// The write routes take a bare view id, so authorisation must come from the row's own
// project — owning some other project must grant nothing here.
func TestSavedViewApi_OwnerOfAnotherProjectCannotTouchThisView(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProjectA := createProject(t, app, owner, "sv-idor-a")
	view := createSavedView(t, app, owner, idProjectA,
		`{"name":"A's view","viewType":"table","config":{"v":1},"isShared":true}`)

	// Owns their own project B, and is not a member of A at all.
	outsider := registerOutsider(t, app, owner, "sv-idor-outsider@test.sk")
	createProject(t, app, outsider, "sv-idor-b")

	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView),
		`{"name":"Stolen","viewType":"table","config":{"v":1},"isShared":true}`, outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	res = Request(t, app, "DELETE", fmt.Sprintf("/api/private/saved-view/%d", view.IdSavedView), "", outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

// A name of blanks would render as an unclickable empty row.
func TestSavedViewApi_RejectsBlankName(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-blankname")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"   ","viewType":"table","config":{"v":1},"isShared":false}`, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestSavedViewApi_TrimsName(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-trimname")

	view := createSavedView(t, app, token, idProject,
		`{"name":"  My bugs  ","viewType":"table","config":{"v":1},"isShared":false}`)
	require.Equal(t, "My bugs", view.Name)
}

func TestSavedViewApi_RejectsUnknownViewType(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-badtype")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Bad","viewType":"timeline","config":{"v":1},"isShared":false}`, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

// Views are per project, so a project id in the path must scope the listing.
func TestSavedViewApi_ListIsScopedToProject(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idA := createProject(t, app, token, "sv-scope-a")
	idB := createProject(t, app, token, "sv-scope-b")
	createSavedView(t, app, token, idA, `{"name":"A","viewType":"table","config":{"v":1},"isShared":true}`)

	require.Len(t, loadSavedViews(t, app, token, idA), 1)
	require.Empty(t, loadSavedViews(t, app, token, idB))
}

func TestSavedViewApi_NonMemberIsForbidden(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-outsider")
	outsider := registerViewer(t, app, owner, createProject(t, app, owner, "sv-other"), "sv-outsider@test.sk")

	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/saved-view", idProject), "", outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	res = Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Nope","viewType":"table","config":{"v":1},"isShared":false}`, outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSavedViewApi_ViewerCannotCreate(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "sv-viewer")
	viewer := registerViewer(t, app, owner, idProject, "sv-viewer@test.sk")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"Nope","viewType":"table","config":{"v":1},"isShared":false}`, viewer)
	require.Equal(t, http.StatusForbidden, res.StatusCode, "creating is member+")

	require.Empty(t, loadSavedViews(t, app, viewer, idProject), "but a viewer may still read")
}

func TestSavedViewApi_UnknownIdIsNotFound(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	res := Request(t, app, "PATCH", "/api/private/saved-view/999999",
		`{"name":"Ghost","viewType":"table","config":{"v":1},"isShared":false}`, token)
	require.Equal(t, http.StatusNotFound, res.StatusCode)

	res = Request(t, app, "DELETE", "/api/private/saved-view/999999", "", token)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

// Shared views sort before private ones, then by name — the listing order the
// client renders without re-sorting.
func TestSavedViewApi_ListOrdersSharedFirstThenByName(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-order")
	createSavedView(t, app, token, idProject, `{"name":"zebra","viewType":"table","config":{"v":1},"isShared":false}`)
	createSavedView(t, app, token, idProject, `{"name":"alpha","viewType":"table","config":{"v":1},"isShared":false}`)
	createSavedView(t, app, token, idProject, `{"name":"team","viewType":"table","config":{"v":1},"isShared":true}`)

	names := []string{}
	for _, view := range loadSavedViews(t, app, token, idProject) {
		names = append(names, view.Name)
	}
	require.Equal(t, []string{"team", "alpha", "zebra"}, names)
}
