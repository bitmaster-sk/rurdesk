package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func attachmentImageLink(uploaded model.AttachmentRes) string {
	return fmt.Sprintf("![%s](attachment:%s)", uploaded.FileName, uploaded.IdAttachment)
}

func postMessageWithText(
	t *testing.T,
	app *issue.Application,
	token string,
	recipientType model.MessageRecipientType,
	idRecipient int64,
	text string,
) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"idRecipient":            idRecipient,
		"idMessageRecipientType": recipientType,
		"message":                text,
	})
	require.NoError(t, err)
	return Request(t, app, http.MethodPost, "/api/private/message", string(body), token)
}

func postMessageWithTextOk(
	t *testing.T,
	app *issue.Application,
	token string,
	recipientType model.MessageRecipientType,
	idRecipient int64,
	text string,
) model.Message {
	t.Helper()
	res := postMessageWithText(t, app, token, recipientType, idRecipient, text)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var msg model.Message
	require.NoError(t, json.Unmarshal([]byte(body), &msg))
	return msg
}

func editMessageText(t *testing.T, app *issue.Application, token string, idMessage int64, text string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"message": text})
	require.NoError(t, err)
	return Request(t, app, http.MethodPatch, fmt.Sprintf("/api/private/message/%d", idMessage), string(body), token)
}

func messageAttachmentLinks(t *testing.T, idMessage int64) []uuid.UUID {
	t.Helper()
	pool, err := injector.GetDb()
	require.NoError(t, err)
	rows, err := pool.Query(context.Background(), `
		SELECT id_attachment FROM messages.message_attachment WHERE id_message = $1 ORDER BY id_attachment
	`, idMessage)
	require.NoError(t, err)
	links, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	require.NoError(t, err)
	return links
}

func TestMessageAttachment_LinksFollowMessageTextOnCreateAndEdit(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "msg-attachment-links")

	first := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "first.png", pngContent)
	second := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "second.txt", []byte("notes"))

	msg := postMessageWithTextOk(t, app, token, model.ProjectRecipientType, idProject,
		"look "+attachmentImageLink(first)+" twice "+attachmentImageLink(first))
	require.Equal(t, []uuid.UUID{first.IdAttachment}, messageAttachmentLinks(t, msg.IdMessage))

	res := editMessageText(t, app, token, msg.IdMessage,
		fmt.Sprintf("now only [%s](attachment:%s)", second.FileName, second.IdAttachment))
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, []uuid.UUID{second.IdAttachment}, messageAttachmentLinks(t, msg.IdMessage))

	res = editMessageText(t, app, token, msg.IdMessage, "no files any more")
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Empty(t, messageAttachmentLinks(t, msg.IdMessage))
}

func TestAttachmentOrphanPurge_DeletesOnlyOldUnlinkedAttachments(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-purge")
	ctx := context.Background()
	repo := injector.GetAttachmentRepository()
	pool, err := injector.GetDb()
	require.NoError(t, err)

	oldOrphan := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "old-orphan.txt", []byte("a"))
	freshOrphan := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "fresh-orphan.txt", []byte("b"))
	oldLinked := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "old-linked.png", pngContent)
	postMessageWithTextOk(t, app, token, model.ProjectRecipientType, idProject, attachmentImageLink(oldLinked))

	for _, id := range []uuid.UUID{oldOrphan.IdAttachment, oldLinked.IdAttachment} {
		_, err := pool.Exec(ctx, `UPDATE files.attachment SET create_at = now() - interval '25 hours' WHERE id_attachment = $1`, id)
		require.NoError(t, err)
	}

	deleted, err := repo.DeleteOrphansOlderThan(ctx, time.Now().UTC().Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	_, err = repo.LoadById(ctx, oldOrphan.IdAttachment)
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
	_, err = repo.LoadById(ctx, freshOrphan.IdAttachment)
	require.NoError(t, err)
	_, err = repo.LoadById(ctx, oldLinked.IdAttachment)
	require.NoError(t, err)
}

func TestMessageAttachment_IssueCommentLinksItsAttachment(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "msg-attachment-issue")
	created := createIssue(t, app, token, idProject, "issue with a screenshot")

	uploaded := uploadAttachmentOk(t, app, token, model.IssueRecipientType, created.IdIssue, "bug.png", pngContent)
	msg := postMessageWithTextOk(t, app, token, model.IssueRecipientType, created.IdIssue, attachmentImageLink(uploaded))
	require.Equal(t, []uuid.UUID{uploaded.IdAttachment}, messageAttachmentLinks(t, msg.IdMessage))
}

func TestMessageAttachment_OtherScopeIsRejectedOnCreate(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "msg-attachment-create-a")
	idOtherProject := createProject(t, app, token, "msg-attachment-create-b")
	created := createIssue(t, app, token, idProject, "issue in project a")

	projectAttachment := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "a.png", pngContent)

	res := postMessageWithText(t, app, token, model.ProjectRecipientType, idOtherProject, attachmentImageLink(projectAttachment))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "ATTACHMENT_LINK_INVALID", errorCode(t, res))

	res = postMessageWithText(t, app, token, model.IssueRecipientType, created.IdIssue, attachmentImageLink(projectAttachment))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode, "a project attachment must not be linked from an issue comment")

	res = postMessageWithText(t, app, token, model.ProjectRecipientType, idOtherProject,
		fmt.Sprintf("![ghost](attachment:%s)", uuid.New()))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	listRes := Request(t, app, http.MethodGet,
		fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=%d", idOtherProject, model.ProjectRecipientType), "", token)
	require.Equal(t, http.StatusOK, listRes.StatusCode)
	var msgs []model.Message
	require.NoError(t, json.NewDecoder(listRes.Body).Decode(&msgs))
	require.Empty(t, msgs, "a rejected message must not be stored")
}

func TestMessageAttachment_OtherScopeIsRejectedOnEdit(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "msg-attachment-edit-a")
	idOtherProject := createProject(t, app, token, "msg-attachment-edit-b")

	ownAttachment := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "own.png", pngContent)
	foreignAttachment := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idOtherProject, "foreign.png", pngContent)
	original := "original " + attachmentImageLink(ownAttachment)
	msg := postMessageWithTextOk(t, app, token, model.ProjectRecipientType, idProject, original)

	res := editMessageText(t, app, token, msg.IdMessage, "edited "+attachmentImageLink(foreignAttachment))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "ATTACHMENT_LINK_INVALID", errorCode(t, res))
	require.Equal(t, []uuid.UUID{ownAttachment.IdAttachment}, messageAttachmentLinks(t, msg.IdMessage))

	listRes := Request(t, app, http.MethodGet,
		fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=%d", idProject, model.ProjectRecipientType), "", token)
	require.Equal(t, http.StatusOK, listRes.StatusCode)
	var msgs []model.Message
	require.NoError(t, json.NewDecoder(listRes.Body).Decode(&msgs))
	require.Len(t, msgs, 1)
	require.Equal(t, original, msgs[0].Message, "a rejected edit must not change the text")
}

func TestMessageAttachment_DirectMessageAcceptsOnlyAuthorsOwnAttachment(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	author := createUserAsAdmin(t, app, adminToken,
		`{"name":"msg-att-dm-author","email":"msg-att-dm-author@test.sk","password":"kreslo"}`)
	createUserAsAdmin(t, app, adminToken,
		`{"name":"msg-att-dm-recipient","email":"msg-att-dm-recipient@test.sk","password":"kreslo"}`)
	idRecipient := idOfUser(t, app, adminToken, "msg-att-dm-recipient@test.sk")
	otherAuthor := createUserAsAdmin(t, app, adminToken,
		`{"name":"msg-att-dm-other","email":"msg-att-dm-other@test.sk","password":"kreslo"}`)

	ownAttachment := uploadAttachmentOk(t, app, author, model.TeammateRecipientType, idRecipient, "own.png", pngContent)
	otherAttachment := uploadAttachmentOk(t, app, otherAuthor, model.TeammateRecipientType, idRecipient, "other.png", pngContent)

	res := postMessageWithText(t, app, author, model.TeammateRecipientType, idRecipient, attachmentImageLink(otherAttachment))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "ATTACHMENT_LINK_INVALID", errorCode(t, res))

	msg := postMessageWithTextOk(t, app, author, model.TeammateRecipientType, idRecipient, attachmentImageLink(ownAttachment))
	require.Equal(t, []uuid.UUID{ownAttachment.IdAttachment}, messageAttachmentLinks(t, msg.IdMessage))

	res = editMessageText(t, app, author, msg.IdMessage, attachmentImageLink(otherAttachment))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
}
