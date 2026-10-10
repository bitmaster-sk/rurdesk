package test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func storeAttachment(ctx context.Context, attachment *model.Attachment, content string) error {
	if err := injector.GetAttachmentRepository().Insert(ctx, attachment); err != nil {
		return err
	}
	return injector.GetAttachmentStorage().Put(ctx, attachment.IdAttachment, strings.NewReader(content))
}

func TestAttachmentStorage_PutThenGetReturnsSameContentAndMetadata(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	idProject := createProject(t, app, token, "attachment-storage")
	ctx := context.Background()

	attachment := &model.Attachment{
		FileName: "notes.txt", MimeType: "text/plain; charset=utf-8", Size: int64(len("hello attachment")),
		AttachmentScope: model.AttachmentScope{IdProject: &idProject}, CreateBy: &idUser,
	}
	require.NoError(t, storeAttachment(ctx, attachment, "hello attachment"))
	require.NotEqual(t, uuid.Nil, attachment.IdAttachment)

	content, err := injector.GetAttachmentStorage().Get(ctx, attachment.IdAttachment)
	require.NoError(t, err)
	data, err := io.ReadAll(content)
	require.NoError(t, err)
	require.NoError(t, content.Close())
	require.Equal(t, "hello attachment", string(data))

	loaded, err := injector.GetAttachmentRepository().LoadById(ctx, attachment.IdAttachment)
	require.NoError(t, err)
	require.Equal(t, "notes.txt", loaded.FileName)
	require.Equal(t, "text/plain; charset=utf-8", loaded.MimeType)
	require.Equal(t, attachment.Size, loaded.Size)
	require.Equal(t, idProject, *loaded.IdProject)
	require.Nil(t, loaded.IdIssue)
	require.Equal(t, idUser, *loaded.CreateBy)
}

func TestAttachmentStorage_UnknownIdIsNotFound(t *testing.T) {
	Setup(t)
	ctx := context.Background()

	_, err := injector.GetAttachmentStorage().Get(ctx, uuid.New())
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)

	_, err = injector.GetAttachmentRepository().LoadById(ctx, uuid.New())
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
}

func TestAttachmentStorage_ScopeMustBeExactlyOne(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	idProject := createProject(t, app, token, "attachment-scope")
	ctx := context.Background()

	require.Error(t, storeAttachment(ctx, &model.Attachment{FileName: "none.txt", MimeType: "text/plain"}, "x"),
		"an attachment without a scope must be rejected")

	require.Error(t, storeAttachment(ctx, &model.Attachment{
		FileName: "two.txt", MimeType: "text/plain",
		AttachmentScope: model.AttachmentScope{IdProject: &idProject, IdUserTo: &idUser},
	}, "x"), "an attachment with two scopes must be rejected")
}

func TestAttachmentStorage_DeletingProjectDeletesItsAttachments(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "attachment-cascade")
	ctx := context.Background()

	attachment := &model.Attachment{
		FileName: "gone.txt", MimeType: "text/plain",
		AttachmentScope: model.AttachmentScope{IdProject: &idProject},
	}
	require.NoError(t, storeAttachment(ctx, attachment, "bye"))

	pool, err := injector.GetDb()
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM projects.project WHERE id_project = $1`, idProject)
	require.NoError(t, err)

	_, err = injector.GetAttachmentRepository().LoadById(ctx, attachment.IdAttachment)
	require.ErrorIs(t, err, repository.ErrAttachmentNotFound)
}
