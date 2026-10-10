package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var pngContent = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x42}, 64)...)

func uploadAttachment(
	t *testing.T,
	app *issue.Application,
	token string,
	recipientType model.MessageRecipientType,
	idRecipient int64,
	fileName string,
	content []byte,
) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("idMessageRecipientType", strconv.Itoa(int(recipientType))))
	require.NoError(t, writer.WriteField("idRecipient", strconv.FormatInt(idRecipient, 10)))
	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/private/attachment", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", token)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, req)
	return recorder.Result()
}

func uploadAttachmentOk(
	t *testing.T,
	app *issue.Application,
	token string,
	recipientType model.MessageRecipientType,
	idRecipient int64,
	fileName string,
	content []byte,
) model.AttachmentRes {
	t.Helper()
	res := uploadAttachment(t, app, token, recipientType, idRecipient, fileName, content)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var uploaded model.AttachmentRes
	require.NoError(t, json.Unmarshal([]byte(body), &uploaded))
	return uploaded
}

func downloadAttachment(t *testing.T, app *issue.Application, token string, idAttachment uuid.UUID) *http.Response {
	t.Helper()
	return Request(t, app, http.MethodGet, "/api/private/attachment/"+idAttachment.String(), "", token)
}

func setAttachmentMaxSizeMb(t *testing.T, app *issue.Application, adminToken string, sizeMb int) {
	t.Helper()
	res := Request(t, app, http.MethodPatch, "/api/private/admin/settings",
		fmt.Sprintf(`{"attachmentMaxSizeMb":%d}`, sizeMb), adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
}

func createTeamAsAdmin(t *testing.T, app *issue.Application, adminToken, name string) int64 {
	t.Helper()
	res := Request(t, app, http.MethodPost, "/api/private/admin/team",
		`{"name":"`+name+`","color":"#00ff00"}`, adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&team))
	return team.IdTeam
}

func TestAttachmentApi_UploadThenDownloadImage(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-image")

	uploaded := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "screen.png", pngContent)
	require.NotEqual(t, uuid.Nil, uploaded.IdAttachment)
	require.Equal(t, "screen.png", uploaded.FileName)
	require.Equal(t, "image/png", uploaded.MimeType)
	require.Equal(t, int64(len(pngContent)), uploaded.Size)

	res := downloadAttachment(t, app, token, uploaded.IdAttachment)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "image/png", res.Header.Get("Content-Type"))
	require.Equal(t, `inline; filename=screen.png`, res.Header.Get("Content-Disposition"))
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "private, max-age=31536000, immutable", res.Header.Get("Cache-Control"))
	require.Equal(t, string(pngContent), readBody(t, res))
}

func TestAttachmentApi_NonPreviewableFileDownloadsAsAttachment(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-text")

	uploaded := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "notes.txt", []byte("plain notes"))
	require.Equal(t, "text/plain; charset=utf-8", uploaded.MimeType)

	res := downloadAttachment(t, app, token, uploaded.IdAttachment)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, `attachment; filename=notes.txt`, res.Header.Get("Content-Disposition"))
	require.Equal(t, "plain notes", readBody(t, res))
}

func TestAttachmentApi_UnknownAttachmentIsNotFound(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	res := downloadAttachment(t, app, token, uuid.New())
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestAttachmentApi_ForeignProjectIsForbidden(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-foreign-project")
	outsider := createUserAsAdmin(t, app, token,
		`{"name":"att-prj-outsider","email":"att-prj-outsider@test.sk","password":"kreslo"}`)

	res := uploadAttachment(t, app, outsider, model.ProjectRecipientType, idProject, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))

	uploaded := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, app, outsider, uploaded.IdAttachment).StatusCode)
}

func TestAttachmentApi_ForeignIssueIsForbidden(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-foreign-issue")
	created := createIssue(t, app, token, idProject, "issue with attachment")
	outsider := createUserAsAdmin(t, app, token,
		`{"name":"att-iss-outsider","email":"att-iss-outsider@test.sk","password":"kreslo"}`)

	res := uploadAttachment(t, app, outsider, model.IssueRecipientType, created.IdIssue, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	uploaded := uploadAttachmentOk(t, app, token, model.IssueRecipientType, created.IdIssue, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, app, outsider, uploaded.IdAttachment).StatusCode)
	require.Equal(t, http.StatusOK, downloadAttachment(t, app, token, uploaded.IdAttachment).StatusCode)
}

func TestAttachmentApi_ForeignTeamIsForbidden(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	idTeam := createTeamAsAdmin(t, app, adminToken, "attachment-api-team")
	member := createUserAsAdmin(t, app, adminToken,
		`{"name":"att-team-member","email":"att-team-member@test.sk","password":"kreslo"}`)
	idMember := idOfUser(t, app, adminToken, "att-team-member@test.sk")
	res := Request(t, app, http.MethodPost, "/api/private/admin/team/member",
		fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, idTeam, idMember), adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	outsider := createUserAsAdmin(t, app, adminToken,
		`{"name":"att-team-outsider","email":"att-team-outsider@test.sk","password":"kreslo"}`)

	res = uploadAttachment(t, app, outsider, model.TeamRecipientType, idTeam, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	uploaded := uploadAttachmentOk(t, app, member, model.TeamRecipientType, idTeam, "x.png", pngContent)
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, app, outsider, uploaded.IdAttachment).StatusCode)
}

func TestAttachmentApi_DirectMessageReadableOnlyByAuthorAndRecipient(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	author := createUserAsAdmin(t, app, adminToken,
		`{"name":"att-dm-author","email":"att-dm-author@test.sk","password":"kreslo"}`)
	recipient := createUserAsAdmin(t, app, adminToken,
		`{"name":"att-dm-recipient","email":"att-dm-recipient@test.sk","password":"kreslo"}`)
	idRecipient := idOfUser(t, app, adminToken, "att-dm-recipient@test.sk")
	stranger := createUserAsAdmin(t, app, adminToken,
		`{"name":"att-dm-stranger","email":"att-dm-stranger@test.sk","password":"kreslo"}`)

	uploaded := uploadAttachmentOk(t, app, author, model.TeammateRecipientType, idRecipient, "dm.png", pngContent)

	require.Equal(t, http.StatusOK, downloadAttachment(t, app, author, uploaded.IdAttachment).StatusCode)
	require.Equal(t, http.StatusOK, downloadAttachment(t, app, recipient, uploaded.IdAttachment).StatusCode)
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, app, stranger, uploaded.IdAttachment).StatusCode)
	require.Equal(t, http.StatusForbidden, downloadAttachment(t, app, adminToken, uploaded.IdAttachment).StatusCode)

	res := uploadAttachment(t, app, author, model.TeammateRecipientType, 999999999, "dm.png", pngContent)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestAttachmentApi_TypeIsDetectedFromContent(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-types")

	rejected := []struct {
		name     string
		fileName string
		content  string
	}{
		{"svg disguised as png", "logo.png", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
		{"svg with xml prolog", "logo.svg", `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`},
		{"html disguised as txt", "page.txt", `<!DOCTYPE html><html><body><script>alert(1)</script></body></html>`},
	}
	for _, testCase := range rejected {
		t.Run(testCase.name, func(t *testing.T) {
			res := uploadAttachment(t, app, token, model.ProjectRecipientType, idProject, testCase.fileName, []byte(testCase.content))
			require.Equal(t, http.StatusUnsupportedMediaType, res.StatusCode)
			require.Equal(t, "ATTACHMENT_TYPE_NOT_ALLOWED", errorCode(t, res))
		})
	}

	t.Run("png named svg is stored as png", func(t *testing.T) {
		uploaded := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "picture.svg", pngContent)
		require.Equal(t, "image/png", uploaded.MimeType)
	})
}

func TestAttachmentApi_MaxSizeSettingIsEnforced(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-api-size")
	t.Cleanup(func() { setAttachmentMaxSizeMb(t, app, token, 25) })

	slightlyOver := bytes.Repeat([]byte("a"), (1<<20)+1024)
	farOver := bytes.Repeat([]byte("a"), 3<<19)

	setAttachmentMaxSizeMb(t, app, token, 1)
	for _, content := range [][]byte{slightlyOver, farOver} {
		res := uploadAttachment(t, app, token, model.ProjectRecipientType, idProject, "big.txt", content)
		require.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
		require.Equal(t, "ATTACHMENT_TOO_LARGE", errorCode(t, res))
	}

	setAttachmentMaxSizeMb(t, app, token, 2)
	uploaded := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "big.txt", farOver)
	require.Equal(t, int64(len(farOver)), uploaded.Size)
}

func TestAttachmentApi_DeletingIssueOrProjectDeletesAttachments(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	ctx := context.Background()
	idProject := createProject(t, app, token, "attachment-api-cascade")
	created := createIssue(t, app, token, idProject, "issue to delete")

	issueAttachment := uploadAttachmentOk(t, app, token, model.IssueRecipientType, created.IdIssue, "issue.png", pngContent)
	projectAttachment := uploadAttachmentOk(t, app, token, model.ProjectRecipientType, idProject, "project.png", pngContent)

	res := Request(t, app, http.MethodDelete,
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, created.IdIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	_, err := injector.GetAttachmentRepository().LoadById(ctx, issueAttachment.IdAttachment)
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
	_, err = injector.GetAttachmentRepository().LoadById(ctx, projectAttachment.IdAttachment)
	require.NoError(t, err)

	res = Request(t, app, http.MethodDelete, fmt.Sprintf("/api/private/project/%d", idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	_, err = injector.GetAttachmentRepository().LoadById(ctx, projectAttachment.IdAttachment)
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
}
