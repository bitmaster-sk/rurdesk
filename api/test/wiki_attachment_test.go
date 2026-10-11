package test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func (f *wikiFixture) projectSpaceId(t *testing.T) int64 {
	t.Helper()
	tree := f.tree(t, f.adminToken)
	require.Len(t, tree.Spaces, 2)
	return tree.Spaces[1].IdSpace
}

func (f *wikiFixture) uploadToSpace(t *testing.T, token string, idSpace int64, fileName string, content []byte) *http.Response {
	t.Helper()
	return uploadAttachmentForm(t, f.app, token, map[string]string{"idSpace": strconv.FormatInt(idSpace, 10)}, fileName, content)
}

func (f *wikiFixture) uploadToSpaceOk(t *testing.T, idSpace int64, fileName string, content []byte) model.AttachmentRes {
	t.Helper()
	return decodeUploaded(t, f.uploadToSpace(t, f.adminToken, idSpace, fileName, content))
}

func wikiAttachmentLinks(t *testing.T, idPage int64) []uuid.UUID {
	t.Helper()
	pool, err := injector.GetDb()
	require.NoError(t, err)
	rows, err := pool.Query(context.Background(), `
		SELECT id_attachment FROM wiki.page_attachment WHERE id_page = $1 ORDER BY id_attachment
	`, idPage)
	require.NoError(t, err)
	links, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	require.NoError(t, err)
	return links
}

func ageAttachmentsAndPurgeOrphans(t *testing.T, idsAttachment ...uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool, err := injector.GetDb()
	require.NoError(t, err)
	for _, idAttachment := range idsAttachment {
		_, err := pool.Exec(ctx, `UPDATE files.attachment SET create_at = now() - interval '25 hours' WHERE id_attachment = $1`, idAttachment)
		require.NoError(t, err)
	}
	_, err = injector.GetAttachmentRepository().DeleteOrphansOlderThan(ctx, time.Now().UTC().Add(-24*time.Hour))
	require.NoError(t, err)
}

func attachmentExists(t *testing.T, idAttachment uuid.UUID) bool {
	t.Helper()
	_, err := injector.GetAttachmentRepository().LoadById(context.Background(), idAttachment)
	if err == nil {
		return true
	}
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
	return false
}

func TestWikiAttachment_UploadNeedsEditAndDownloadNeedsRead(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-acl")
	idSpace := f.projectSpaceId(t)
	viewer := f.userWithRole(t, "wiki-att-viewer@test.sk", "viewer")
	member := f.userWithRole(t, "wiki-att-member@test.sk", "member")
	stranger := f.userWithRole(t, "wiki-att-stranger@test.sk", "")

	uploaded := decodeUploaded(t, f.uploadToSpace(t, member, idSpace, "diagram.png", pngContent))
	require.Equal(t, "image/png", uploaded.MimeType)

	require.Equal(t, http.StatusForbidden, f.uploadToSpace(t, viewer, idSpace, "x.png", pngContent).StatusCode)
	bothScopes := uploadAttachmentForm(t, f.app, member, map[string]string{
		"idSpace":                strconv.FormatInt(idSpace, 10),
		"idMessageRecipientType": strconv.Itoa(int(model.ProjectRecipientType)),
		"idRecipient":            strconv.FormatInt(f.idProject, 10),
	}, "x.png", pngContent)
	require.Equal(t, http.StatusBadRequest, bothScopes.StatusCode)
	require.Equal(t, http.StatusForbidden, f.uploadToSpace(t, stranger, idSpace, "x.png", pngContent).StatusCode)

	res := downloadAttachment(t, f.app, viewer, uploaded.IdAttachment)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, string(pngContent), readBody(t, res))
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, f.app, stranger, uploaded.IdAttachment).StatusCode)
}

func TestWikiAttachment_AgentsCannotUpload(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-agent")
	agent := f.agentUser(t, "wiki-att-agent@test.sk", "member")

	require.Equal(t, http.StatusForbidden, f.uploadToSpace(t, agent, f.projectSpaceId(t), "x.png", pngContent).StatusCode)
}

func TestWikiAttachment_LinkFromAnotherSpaceIsRejected(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-scope")
	other := f.otherProject(t, "wiki-attachment-scope-other")
	foreign := f.uploadToSpaceOk(t, other.projectSpaceId(t), "foreign.png", pngContent)
	chat := uploadAttachmentOk(t, f.app, f.adminToken, model.ProjectRecipientType, f.idProject, "chat.png", pngContent)

	for _, attachment := range []model.AttachmentRes{foreign, chat} {
		res, _ := f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Foreign"), Body: attachmentImageLink(attachment)})
		require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	}

	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Scoped"})
	res := f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Scoped", Body: attachmentImageLink(foreign), AgentAccess: "on_demand"})
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	draftRes := Request(t, f.app, "PUT", fmt.Sprintf("/api/private/wiki/page/%d/draft", page.IdPage),
		fmt.Sprintf(`{"baseVersion":1,"title":"Scoped","body":%q}`, attachmentImageLink(foreign)), f.adminToken)
	require.Equal(t, http.StatusNoContent, draftRes.StatusCode, "a draft keeps saving and only skips the foreign link")
	require.Empty(t, wikiAttachmentLinks(t, page.IdPage))
}

func TestWikiAttachment_RevertedVersionKeepsItsImage(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-revert")
	image := f.uploadToSpaceOk(t, f.projectSpaceId(t), "old.png", pngContent)
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Revert", Body: "see " + attachmentImageLink(image)})
	require.Equal(t, []uuid.UUID{image.IdAttachment}, wikiAttachmentLinks(t, page.IdPage))

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Revert", Body: "no image", AgentAccess: "on_demand"}).StatusCode)
	require.Equal(t, []uuid.UUID{image.IdAttachment}, wikiAttachmentLinks(t, page.IdPage), "saving never drops a link")

	ageAttachmentsAndPurgeOrphans(t, image.IdAttachment)
	require.True(t, attachmentExists(t, image.IdAttachment))

	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/revert/1", page.IdPage), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, http.StatusOK, downloadAttachment(t, f.app, f.adminToken, image.IdAttachment).StatusCode)
}

func TestWikiAttachment_DraftKeepsItsAttachment(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-draft")
	image := f.uploadToSpaceOk(t, f.projectSpaceId(t), "draft.png", pngContent)
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Draft"})

	res := Request(t, f.app, "PUT", fmt.Sprintf("/api/private/wiki/page/%d/draft", page.IdPage),
		fmt.Sprintf(`{"baseVersion":1,"title":"Draft","body":%q}`, attachmentImageLink(image)), f.adminToken)
	require.Equal(t, http.StatusNoContent, res.StatusCode, readBody(t, res))

	ageAttachmentsAndPurgeOrphans(t, image.IdAttachment)
	require.True(t, attachmentExists(t, image.IdAttachment))
}

func TestWikiAttachment_PurgingThePageDropsItsLinks(t *testing.T) {
	f := newWikiFixture(t, "wiki-attachment-purge")
	image := f.uploadToSpaceOk(t, f.projectSpaceId(t), "gone.png", pngContent)
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Purge", Body: attachmentImageLink(image)})

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", page.IdPage), "", f.adminToken).StatusCode)
	ageAttachmentsAndPurgeOrphans(t, image.IdAttachment)
	require.True(t, attachmentExists(t, image.IdAttachment), "a page in the trash still holds its attachments")

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d/purge", page.IdPage), "", f.adminToken).StatusCode)
	require.Empty(t, wikiAttachmentLinks(t, page.IdPage))
	ageAttachmentsAndPurgeOrphans(t)
	require.False(t, attachmentExists(t, image.IdAttachment))
}
