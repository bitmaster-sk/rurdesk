package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func createIssueWithDescription(t *testing.T, app *issue.Application, token string, idProject int64, description string) *http.Response {
	t.Helper()
	body, err := json.Marshal(model.CreateIssueReq{IdProject: idProject, Title: "issue with files", Description: description})
	require.NoError(t, err)
	return Request(t, app, http.MethodPost, fmt.Sprintf("/api/private/project/%d/issue", idProject), string(body), token)
}

func createIssueWithDescriptionOk(t *testing.T, app *issue.Application, token string, idProject int64, description string) model.Issue {
	t.Helper()
	res := createIssueWithDescription(t, app, token, idProject, description)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var created model.Issue
	require.NoError(t, json.Unmarshal([]byte(body), &created))
	return created
}

func editIssueDescription(t *testing.T, app *issue.Application, token string, created model.Issue, description string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"description": description})
	require.NoError(t, err)
	return Request(t, app, http.MethodPatch,
		fmt.Sprintf("/api/private/project/%d/issue/%d", created.IdProject, created.IdIssuePublic), string(body), token)
}

func issueAttachmentLinks(t *testing.T, idIssue int64) []uuid.UUID {
	t.Helper()
	pool, err := injector.GetDb()
	require.NoError(t, err)
	rows, err := pool.Query(context.Background(), `
		SELECT id_attachment FROM issues.issue_attachment WHERE id_issue = $1 ORDER BY id_attachment
	`, idIssue)
	require.NoError(t, err)
	links, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	require.NoError(t, err)
	return links
}

func TestIssueAttachment_DescriptionLinksFollowCreateAndEdit(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-attachment-links")

	draft := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "draft.png", pngContent)
	created := createIssueWithDescriptionOk(t, app, token, idProject, "see "+attachmentImageLink(draft))
	require.Equal(t, []uuid.UUID{draft.IdAttachment}, issueAttachmentLinks(t, created.IdIssue))

	later := uploadAttachmentOk(t, app, token, model.IssueRecipientType, created.IdIssue, "later.txt", []byte("notes"))
	res := editIssueDescription(t, app, token, created, fmt.Sprintf("now [%s](attachment:%s)", later.FileName, later.IdAttachment))
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, []uuid.UUID{later.IdAttachment}, issueAttachmentLinks(t, created.IdIssue))

	res = Request(t, app, http.MethodPatch,
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, created.IdIssuePublic), `{"title":"renamed"}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, []uuid.UUID{later.IdAttachment}, issueAttachmentLinks(t, created.IdIssue), "editing other fields keeps the links")
}

func TestIssueAttachment_LinkFromAnotherScopeIsRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-attachment-scope")
	idOtherProject := createProject(t, app, token, "issue-attachment-scope-other")
	otherIssue := createIssue(t, app, token, idProject, "other issue")

	foreignProject := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idOtherProject, "foreign.png", pngContent)
	foreignIssue := uploadAttachmentOk(t, app, token, model.IssueRecipientType, otherIssue.IdIssue, "sibling.png", pngContent)

	res := createIssueWithDescription(t, app, token, idProject, attachmentImageLink(foreignProject))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	created := createIssueWithDescriptionOk(t, app, token, idProject, "plain")
	for _, foreign := range []model.AttachmentRes{foreignProject, foreignIssue} {
		res = editIssueDescription(t, app, token, created, attachmentImageLink(foreign))
		require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	}
	require.Empty(t, issueAttachmentLinks(t, created.IdIssue))
}

func TestIssueAttachment_McpCreateAndUpdateLinkTheDescription(t *testing.T) {
	f := newWikiFixture(t, "issue-attachment-mcp")
	tools := &wikiAgentFixture{wikiFixture: f, mcpSessions: map[string]string{}}
	first := uploadAttachmentOk(t, f.app, f.adminToken, model.ProjectRecipientType, f.idProject, "first.png", pngContent)
	second := uploadAttachmentOk(t, f.app, f.adminToken, model.ProjectRecipientType, f.idProject, "second.png", pngContent)

	result := tools.callTool(t, f.adminToken, "", "create_issue", map[string]any{
		"project_id": f.idProject, "title": "from mcp", "description": attachmentImageLink(first),
	})
	require.False(t, result.IsError, result.Text)
	var created model.Issue
	require.NoError(t, json.Unmarshal([]byte(result.Text), &created))
	require.Equal(t, []uuid.UUID{first.IdAttachment}, issueAttachmentLinks(t, created.IdIssue))

	result = tools.callTool(t, f.adminToken, "", "update_issue", map[string]any{
		"project_id": f.idProject, "issue_id": created.IdIssuePublic, "description": attachmentImageLink(second),
	})
	require.False(t, result.IsError, result.Text)
	require.Equal(t, []uuid.UUID{second.IdAttachment}, issueAttachmentLinks(t, created.IdIssue))
}

func TestIssueAttachment_DescriptionLinkKeepsTheFileUntilRemoved(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-attachment-purge")

	image := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "kept.png", pngContent)
	created := createIssueWithDescriptionOk(t, app, token, idProject, attachmentImageLink(image))

	ageAttachmentsAndPurgeOrphans(t, image.IdAttachment)
	require.True(t, attachmentExists(t, image.IdAttachment))

	res := editIssueDescription(t, app, token, created, "the image is gone")
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	ageAttachmentsAndPurgeOrphans(t)
	require.False(t, attachmentExists(t, image.IdAttachment))
}

func TestIssueAttachment_DeletingTheIssueDropsLinksAndItsFiles(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-attachment-delete")

	created := createIssueWithDescriptionOk(t, app, token, idProject, "plain")
	image := uploadAttachmentOk(t, app, token, model.IssueRecipientType, created.IdIssue, "issue.png", pngContent)
	res := editIssueDescription(t, app, token, created, attachmentImageLink(image))
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Len(t, issueAttachmentLinks(t, created.IdIssue), 1)

	res = Request(t, app, http.MethodDelete,
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, created.IdIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Empty(t, issueAttachmentLinks(t, created.IdIssue))
	require.False(t, attachmentExists(t, image.IdAttachment))
}

func TestIssueAttachment_ABadLinkAlreadyInTheDescriptionDoesNotBlockEdits(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "issue-attachment-dead-link")
	idOtherProject := createProject(t, app, token, "issue-attachment-dead-link-other")
	created := createIssueWithDescriptionOk(t, app, token, idProject, "plain")
	foreign := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idOtherProject, "foreign.png", pngContent)
	pool, err := injector.GetDb()
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE issues.issue SET description = $1 WHERE id_issue = $2`,
		"copied "+attachmentImageLink(foreign), created.IdIssue)
	require.NoError(t, err)

	res := Request(t, app, http.MethodPatch,
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, created.IdIssuePublic),
		fmt.Sprintf(`{"title":"renamed","description":%q}`, "copied "+attachmentImageLink(foreign)), token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	res = editIssueDescription(t, app, token, created, "copied "+attachmentImageLink(foreign)+" and more text")
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Empty(t, issueAttachmentLinks(t, created.IdIssue))
}
