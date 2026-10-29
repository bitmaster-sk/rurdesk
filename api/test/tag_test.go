package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/stretchr/testify/require"
)

func loadTags(t *testing.T, app *issue.Application, token string, idProject int64) []model.Tag {
	res := Request(t, app, "GET", "/api/private/tag", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.Tag
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	mine := make([]model.Tag, 0, len(all))
	for _, tag := range all {
		if tag.IdProject == idProject {
			mine = append(mine, tag)
		}
	}
	return mine
}

func postTag(t *testing.T, app *issue.Application, token string, idProject int64, name string) *http.Response {
	body, _ := json.Marshal(model.CreateTagReq{IdProject: idProject, Name: name, Color: "#16a34a"})
	return Request(t, app, "POST", "/api/private/tag", string(body), token)
}

func createTag(t *testing.T, app *issue.Application, token string, idProject int64, name string) model.Tag {
	res := postTag(t, app, token, idProject, name)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var created model.Tag
	require.Nil(t, json.Unmarshal([]byte(body), &created))
	return created
}

func editTag(t *testing.T, app *issue.Application, token string, tag model.Tag, name string) *http.Response {
	body, _ := json.Marshal(model.EditTagReq{IdProject: tag.IdProject, Name: name, Color: tag.Color, OrderRank: tag.OrderRank})
	return Request(t, app, "PATCH", "/api/private/tag/"+itoa(tag.IdTag), string(body), token)
}

func createIssueWithTags(t *testing.T, app *issue.Application, token string, idProject int64, title string, idsTag []int64) model.Issue {
	body, _ := json.Marshal(model.CreateIssueReq{IdProject: idProject, Title: title, Description: "x", IdsTag: idsTag})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	resBody := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, resBody)
	var created model.Issue
	require.Nil(t, json.Unmarshal([]byte(resBody), &created))
	return created
}

func loadIssueTags(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64) []int64 {
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, idIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var loaded model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&loaded))
	return loaded.IdsTag
}

func memberToken(t *testing.T, app *issue.Application, adminToken string, idProject int64, email string) string {
	token := createUserAsAdmin(t, app, adminToken,
		fmt.Sprintf(`{"name":"%s","email":"%s","password":"secret1"}`, email, email))
	addMember(t, app, adminToken, idProject, idOfUser(t, app, adminToken, email))
	return token
}

func TestTag_CreateAppendsAtEndWithZeroUsage(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-create")

	first := createTag(t, app, token, idProject, "next")
	second := createTag(t, app, token, idProject, "  gateway  ")

	require.Equal(t, 1, first.OrderRank)
	require.Equal(t, 2, second.OrderRank)
	require.Equal(t, "gateway", second.Name)

	tags := loadTags(t, app, token, idProject)
	require.Len(t, tags, 2)
	require.Equal(t, 0, tags[0].IssueCount)
}

func TestTag_DuplicateNameIgnoringCaseIsConflict(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-duplicate")
	createTag(t, app, token, idProject, "bug")
	other := createTag(t, app, token, idProject, "wiki")

	res := postTag(t, app, token, idProject, "Bug")
	require.Equal(t, http.StatusConflict, res.StatusCode)

	res = editTag(t, app, token, other, "BUG")
	require.Equal(t, http.StatusConflict, res.StatusCode)
	require.Len(t, loadTags(t, app, token, idProject), 2)
}

func TestTag_SameNameInAnotherProjectIsAllowed(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	createTag(t, app, token, createProject(t, app, token, "tag-scope-a"), "next")
	createTag(t, app, token, createProject(t, app, token, "tag-scope-b"), "next")
}

func TestTag_RenameKeepsEveryTaggedIssue(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-rename")
	next := createTag(t, app, token, idProject, "next")
	first := createIssueWithTags(t, app, token, idProject, "first", []int64{next.IdTag})
	second := createIssueWithTags(t, app, token, idProject, "second", []int64{next.IdTag})

	res := editTag(t, app, token, next, "v1.6.0")
	require.Equal(t, http.StatusOK, res.StatusCode)

	tags := loadTags(t, app, token, idProject)
	require.Len(t, tags, 1)
	require.Equal(t, "v1.6.0", tags[0].Name)
	require.Equal(t, 2, tags[0].IssueCount)
	require.Equal(t, []int64{next.IdTag}, loadIssueTags(t, app, token, idProject, first.IdIssuePublic))
	require.Equal(t, []int64{next.IdTag}, loadIssueTags(t, app, token, idProject, second.IdIssuePublic))
}

func TestTag_DeleteRemovesItFromIssuesAndKeepsTheIssues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-delete")
	next := createTag(t, app, token, idProject, "next")
	wiki := createTag(t, app, token, idProject, "wiki")
	tagged := createIssueWithTags(t, app, token, idProject, "tagged", []int64{next.IdTag, wiki.IdTag})

	res := Request(t, app, "DELETE", "/api/private/tag/"+itoa(next.IdTag)+"/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.Equal(t, []int64{wiki.IdTag}, loadIssueTags(t, app, token, idProject, tagged.IdIssuePublic))
	tags := loadTags(t, app, token, idProject)
	require.Len(t, tags, 1)
	require.Equal(t, 1, tags[0].OrderRank)
}

func TestTag_PatchWithoutTagsKeepsThemAndEmptyListClears(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-patch")
	next := createTag(t, app, token, idProject, "next")
	created := createIssueWithTags(t, app, token, idProject, "patched", []int64{next.IdTag})

	res := patchIssue(t, app, token, idProject, created.IdIssuePublic, `{"title":"renamed"}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, []int64{next.IdTag}, loadIssueTags(t, app, token, idProject, created.IdIssuePublic))

	res = patchIssue(t, app, token, idProject, created.IdIssuePublic, `{"idsTag":[]}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Empty(t, loadIssueTags(t, app, token, idProject, created.IdIssuePublic))
}

func TestTag_PatchReplacesTheTagList(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-patch-replace")
	next := createTag(t, app, token, idProject, "next")
	wiki := createTag(t, app, token, idProject, "wiki")
	created := createIssueWithTags(t, app, token, idProject, "replaced", []int64{next.IdTag})

	res := patchIssue(t, app, token, idProject, created.IdIssuePublic, fmt.Sprintf(`{"idsTag":[%d,%d]}`, wiki.IdTag, wiki.IdTag))
	require.Equal(t, http.StatusOK, res.StatusCode)
	var patched model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&patched))
	require.Equal(t, []int64{wiki.IdTag}, patched.IdsTag)
	require.Equal(t, []int64{wiki.IdTag}, loadIssueTags(t, app, token, idProject, created.IdIssuePublic))
}

func TestTag_FromAnotherProjectCannotBeAssigned(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-foreign")
	foreign := createTag(t, app, token, createProject(t, app, token, "tag-foreign-other"), "next")

	body, _ := json.Marshal(model.CreateIssueReq{IdProject: idProject, Title: "x", Description: "x", IdsTag: []int64{foreign.IdTag}})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	created := createIssueWithTags(t, app, token, idProject, "plain", nil)
	res = patchIssue(t, app, token, idProject, created.IdIssuePublic, fmt.Sprintf(`{"idsTag":[%d]}`, foreign.IdTag))
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.Empty(t, loadIssueTags(t, app, token, idProject, created.IdIssuePublic))

	bulk := fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"addIdsTag":[%d]}]}`, created.IdIssuePublic, foreign.IdTag)
	res = Request(t, app, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/batch", idProject), bulk, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.Contains(t, readBody(t, res), "TAG_NOT_IN_PROJECT")
	require.Empty(t, loadIssueTags(t, app, token, idProject, created.IdIssuePublic))
}

func TestTag_FilterMatchesAnyOfTheSelectedTags(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-filter")
	next := createTag(t, app, token, idProject, "next")
	wiki := createTag(t, app, token, idProject, "wiki")
	agentTag := createTag(t, app, token, idProject, "agent")
	withNext := createIssueWithTags(t, app, token, idProject, "next", []int64{next.IdTag})
	withWiki := createIssueWithTags(t, app, token, idProject, "wiki", []int64{wiki.IdTag})
	createIssueWithTags(t, app, token, idProject, "agent", []int64{agentTag.IdTag})
	untagged := createIssueWithTags(t, app, token, idProject, "none", nil)

	base := fmt.Sprintf("/api/private/project/%d/issue?idsTag=%d,%d", idProject, next.IdTag, wiki.IdTag)

	flat := loadIssuesPage(t, app, token, base)
	require.ElementsMatch(t, []int64{withNext.IdIssuePublic, withWiki.IdIssuePublic}, publicIds(flat))

	paged := loadIssuesPage(t, app, token, base+"&limit=10")
	require.ElementsMatch(t, []int64{withNext.IdIssuePublic, withWiki.IdIssuePublic}, publicIds(paged))
	require.Equal(t, 2, paged.Total)

	withUnset := loadIssuesPage(t, app, token, base+"&tagUnset=true")
	require.ElementsMatch(t, []int64{withNext.IdIssuePublic, withWiki.IdIssuePublic, untagged.IdIssuePublic}, publicIds(withUnset))
}

func TestTag_EveryListShapeCarriesTheTags(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-shapes")
	next := createTag(t, app, token, idProject, "next")
	createIssueWithTags(t, app, token, idProject, "tagged", []int64{next.IdTag})

	paged := loadIssuesPage(t, app, token, fmt.Sprintf("/api/private/project/%d/issue?limit=10", idProject))
	require.Len(t, paged.Items, 1)
	require.Equal(t, []int64{next.IdTag}, paged.Items[0].IdsTag)

	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/issue?groupBy=state", idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var grouped struct {
		Groups []model.IssueGroupRes `json:"groups"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&grouped))
	require.Len(t, grouped.Groups, 1)
	require.Equal(t, []int64{next.IdTag}, grouped.Groups[0].Items[0].IdsTag)
}

func TestTag_BulkAddAndRemoveLeaveOtherTagsAlone(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-bulk")
	next := createTag(t, app, token, idProject, "next")
	wiki := createTag(t, app, token, idProject, "wiki")
	agentTag := createTag(t, app, token, idProject, "agent")
	first := createIssueWithTags(t, app, token, idProject, "first", []int64{wiki.IdTag, agentTag.IdTag})
	second := createIssueWithTags(t, app, token, idProject, "second", []int64{agentTag.IdTag})
	untouched := createIssueWithTags(t, app, token, idProject, "untouched", []int64{agentTag.IdTag})

	body := fmt.Sprintf(`{"issues":[
		{"idIssuePublic":%d,"addIdsTag":[%d],"removeIdsTag":[%d]},
		{"idIssuePublic":%d,"addIdsTag":[%d],"removeIdsTag":[%d]}
	]}`, first.IdIssuePublic, next.IdTag, agentTag.IdTag, second.IdIssuePublic, next.IdTag, agentTag.IdTag)
	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/batch", idProject), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	require.ElementsMatch(t, []int64{next.IdTag, wiki.IdTag}, loadIssueTags(t, app, token, idProject, first.IdIssuePublic))
	require.Equal(t, []int64{next.IdTag}, loadIssueTags(t, app, token, idProject, second.IdIssuePublic))
	require.Equal(t, []int64{agentTag.IdTag}, loadIssueTags(t, app, token, idProject, untouched.IdIssuePublic))
}

func TestTag_MemberAssignsExistingTagsButCannotManageThem(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	idProject := createProject(t, app, adminToken, "tag-acl-member")
	next := createTag(t, app, adminToken, idProject, "next")
	created := createIssueWithTags(t, app, adminToken, idProject, "shared", nil)
	token := memberToken(t, app, adminToken, idProject, "tag-member@test.sk")

	require.Equal(t, http.StatusForbidden, postTag(t, app, token, idProject, "mine").StatusCode)
	require.Equal(t, http.StatusForbidden, editTag(t, app, token, next, "renamed").StatusCode)
	res := Request(t, app, "DELETE", "/api/private/tag/"+itoa(next.IdTag)+"/project/"+itoa(idProject), "", token)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	require.Len(t, loadTags(t, app, token, idProject), 1)
	res = patchIssue(t, app, token, idProject, created.IdIssuePublic, fmt.Sprintf(`{"idsTag":[%d]}`, next.IdTag))
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, []int64{next.IdTag}, loadIssueTags(t, app, adminToken, idProject, created.IdIssuePublic))
}

func TestTag_OutsiderNeitherSeesNorChangesProjectTags(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	idProject := createProject(t, app, adminToken, "tag-acl-outsider")
	next := createTag(t, app, adminToken, idProject, "next")
	token := createUserAsAdmin(t, app, adminToken, `{"name":"tag-outsider","email":"tag-outsider@test.sk","password":"secret1"}`)

	require.Empty(t, loadTags(t, app, token, idProject))
	require.Equal(t, http.StatusForbidden, editTag(t, app, token, next, "hijacked").StatusCode)
	require.Equal(t, "next", loadTags(t, app, adminToken, idProject)[0].Name)
}

func TestTag_IssueUpdateNoticeCarriesTags(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "tag-notice")
	next := createTag(t, app, token, idProject, "next")
	created := createIssueWithTags(t, app, token, idProject, "noticed", []int64{next.IdTag})

	notifier := captureNotifier()
	agent.BroadcastIssueUpdate(context.Background(), notifier,
		injector.GetIssueRepository(), injector.GetProjectRepository(), created.IdIssue)

	notice := drainNotice(t, notifier)
	require.Equal(t, notify.SubjectIssue, notice.Subject)
	issue, ok := notice.Payload.(*model.Issue)
	require.True(t, ok)
	require.Equal(t, []int64{next.IdTag}, issue.IdsTag)
}

func TestTagNotice_ReachesProjectMembersOnly(t *testing.T) {
	app := Setup(t)
	ownerToken := Token(t, app)
	idProject := createProject(t, app, ownerToken, "tag-notice-members")
	tag := createTag(t, app, ownerToken, idProject, "next")

	createUserAndLogin(t, app, ownerToken, "tag-notice-stranger@test.sk")
	var idStranger int64
	require.Nil(t, app.Pool.QueryRow(context.Background(),
		`SELECT id_user FROM users.user WHERE email = 'tag-notice-stranger@test.sk'`).Scan(&idStranger))

	notifier := captureNotifier()
	controller.NewTagController(nil, nil, nil, notifier, injector.GetProjectMemberService()).
		BroadcastTag(context.Background(), &tag, notify.ActionDelete)

	notice := drainNotice(t, notifier)
	require.Equal(t, notify.SubjectTag, notice.Subject)
	require.Equal(t, notify.ActionDelete, notice.Action)
	require.NotEmpty(t, notice.IdsUser)
	require.NotContains(t, notice.IdsUser, idStranger)

	payload, ok := notice.Payload.(*model.Tag)
	require.True(t, ok, "tag notice must carry *model.Tag")
	require.Equal(t, tag.IdTag, payload.IdTag)
}
