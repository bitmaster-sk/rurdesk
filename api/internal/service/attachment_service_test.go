package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeAttachmentLinker struct {
	attachments map[uuid.UUID]*model.Attachment
	links       map[int64][]uuid.UUID
}

func (f *fakeAttachmentLinker) Insert(_ context.Context, attachment *model.Attachment) error {
	f.attachments[attachment.IdAttachment] = attachment
	return nil
}

func (f *fakeAttachmentLinker) LoadById(_ context.Context, idAttachment uuid.UUID) (*model.Attachment, error) {
	attachment, ok := f.attachments[idAttachment]
	if !ok {
		return nil, repository.ErrAttachmentNotFound
	}
	return attachment, nil
}

func (f *fakeAttachmentLinker) LoadByIds(_ context.Context, idsAttachment []uuid.UUID) ([]*model.Attachment, error) {
	var found []*model.Attachment
	for _, idAttachment := range idsAttachment {
		if attachment, ok := f.attachments[idAttachment]; ok {
			found = append(found, attachment)
		}
	}
	return found, nil
}

func (f *fakeAttachmentLinker) ReplaceMessageLinks(_ context.Context, idMessage int64, idsAttachment []uuid.UUID) error {
	f.links[idMessage] = idsAttachment
	return nil
}

func TestAttachmentService_LinkMessageAttachments(t *testing.T) {
	const idAuthor, idOther, idMessage = int64(1), int64(2), int64(500)
	idProject, idOtherProject, idRecipient := int64(20), int64(21), int64(7)
	author, other := idAuthor, idOther

	projectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdProject: &idProject}, CreateBy: &other}
	foreignAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdProject: &idOtherProject}, CreateBy: &author}
	ownDirectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdUserTo: &idRecipient}, CreateBy: &author}
	otherDirectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdUserTo: &idRecipient}, CreateBy: &other}

	projectScope := model.AttachmentScope{IdProject: &idProject}
	directScope := model.AttachmentScope{IdUserTo: &idRecipient}

	testCases := []struct {
		name          string
		scope         model.AttachmentScope
		idsAttachment []uuid.UUID
		wantErr       error
	}{
		{"same scope by another member is linked", projectScope, []uuid.UUID{projectAttachment.IdAttachment}, nil},
		{"attachment from another scope is rejected", projectScope, []uuid.UUID{projectAttachment.IdAttachment, foreignAttachment.IdAttachment}, errs.ErrAttachmentLinkInvalid},
		{"unknown attachment is rejected", projectScope, []uuid.UUID{uuid.New()}, errs.ErrAttachmentLinkInvalid},
		{"own direct message attachment is linked", directScope, []uuid.UUID{ownDirectAttachment.IdAttachment}, nil},
		{"direct message attachment of another author is rejected", directScope, []uuid.UUID{otherDirectAttachment.IdAttachment}, errs.ErrAttachmentLinkInvalid},
		{"no links clears the message links", projectScope, nil, nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			linker := &fakeAttachmentLinker{
				attachments: map[uuid.UUID]*model.Attachment{},
				links:       map[int64][]uuid.UUID{idMessage: {uuid.New()}},
			}
			for _, attachment := range []*model.Attachment{projectAttachment, foreignAttachment, ownDirectAttachment, otherDirectAttachment} {
				linker.attachments[attachment.IdAttachment] = attachment
			}
			svc := service.NewAttachmentService(nil, nil, linker, nil, nil)

			recipientType, idRecipient, _ := testCase.scope.Recipient()
			msg := &model.Message{
				IdMessage:              idMessage,
				IdMessageRecipientType: recipientType,
				IdRecipient:            idRecipient,
				Creator:                &model.User{IdUser: idAuthor},
				Message:                attachmentLinks(testCase.idsAttachment),
			}

			err := svc.LinkMessageAttachments(context.Background(), msg)

			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				require.Len(t, linker.links[idMessage], 1, "links must stay untouched when the message is rejected")
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.idsAttachment, linker.links[idMessage])
		})
	}
}

func attachmentLinks(idsAttachment []uuid.UUID) string {
	links := make([]string, 0, len(idsAttachment))
	for _, idAttachment := range idsAttachment {
		links = append(links, "[file](attachment:"+idAttachment.String()+")")
	}
	return strings.Join(links, " ")
}
