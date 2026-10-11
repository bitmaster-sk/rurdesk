package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeAttachmentLinker struct {
	attachments map[uuid.UUID]*model.Attachment
	links       map[int64][]uuid.UUID
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

func (f *fakeAttachmentLinker) AddWikiLinks(_ context.Context, idPage int64, idsAttachment []uuid.UUID) error {
	f.links[idPage] = append(f.links[idPage], idsAttachment...)
	return nil
}

func (f *fakeAttachmentLinker) ReplaceIssueLinks(_ context.Context, idIssue int64, idsAttachment []uuid.UUID) error {
	f.links[idIssue] = idsAttachment
	return nil
}

func newFakeAttachmentLinker(idOwner int64, attachments ...*model.Attachment) *fakeAttachmentLinker {
	linker := &fakeAttachmentLinker{
		attachments: map[uuid.UUID]*model.Attachment{},
		links:       map[int64][]uuid.UUID{idOwner: {uuid.New()}},
	}
	for _, attachment := range attachments {
		linker.attachments[attachment.IdAttachment] = attachment
	}
	return linker
}

func TestAttachmentLinkService_LinkMessageAttachments(t *testing.T) {
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
			linker := newFakeAttachmentLinker(idMessage, projectAttachment, foreignAttachment, ownDirectAttachment, otherDirectAttachment)
			svc := service.NewAttachmentLinkService(linker)

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

func TestAttachmentLinkService_LinkWikiAttachments(t *testing.T) {
	const idPage = int64(300)
	idSpace, idOtherSpace, idProject := int64(4), int64(5), int64(20)

	spaceAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdSpace: &idSpace}}
	otherSpaceAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdSpace: &idOtherSpace}}
	projectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdProject: &idProject}}

	testCases := []struct {
		name          string
		idsAttachment []uuid.UUID
		wantErr       error
	}{
		{"attachment of the same space is linked", []uuid.UUID{spaceAttachment.IdAttachment}, nil},
		{"attachment of another space is rejected", []uuid.UUID{otherSpaceAttachment.IdAttachment}, errs.ErrAttachmentLinkInvalid},
		{"attachment of a project chat is rejected", []uuid.UUID{projectAttachment.IdAttachment}, errs.ErrAttachmentLinkInvalid},
		{"unknown attachment is rejected", []uuid.UUID{uuid.New()}, errs.ErrAttachmentLinkInvalid},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			linker := newFakeAttachmentLinker(idPage, spaceAttachment, otherSpaceAttachment, projectAttachment)
			previous := linker.links[idPage]

			err := service.NewAttachmentLinkService(linker).LinkWikiAttachments(context.Background(), idSpace, idPage, attachmentLinks(testCase.idsAttachment))

			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				require.Equal(t, previous, linker.links[idPage])
				return
			}
			require.NoError(t, err)
			require.Equal(t, append(previous, testCase.idsAttachment...), linker.links[idPage], "earlier links must be kept")
		})
	}
}

func TestAttachmentLinkService_LinkWikiDraftAttachmentsSkipsForeignLinks(t *testing.T) {
	const idPage = int64(300)
	idSpace, idOtherSpace := int64(4), int64(5)
	spaceAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdSpace: &idSpace}}
	otherSpaceAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdSpace: &idOtherSpace}}
	linker := newFakeAttachmentLinker(idPage, spaceAttachment, otherSpaceAttachment)
	previous := linker.links[idPage]

	body := attachmentLinks([]uuid.UUID{otherSpaceAttachment.IdAttachment, uuid.New(), spaceAttachment.IdAttachment})
	err := service.NewAttachmentLinkService(linker).LinkWikiDraftAttachments(context.Background(), idSpace, idPage, body)

	require.NoError(t, err)
	require.Equal(t, append(previous, spaceAttachment.IdAttachment), linker.links[idPage])
}

func TestAttachmentLinkService_LinkIssueAttachments(t *testing.T) {
	const idIssue = int64(400)
	idProject, idOtherProject, idOtherIssue, issueOfAttachment := int64(20), int64(21), int64(401), idIssue

	issueAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdIssue: &issueOfAttachment}}
	projectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdProject: &idProject}}
	otherIssueAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdIssue: &idOtherIssue}}
	otherProjectAttachment := &model.Attachment{IdAttachment: uuid.New(), AttachmentScope: model.AttachmentScope{IdProject: &idOtherProject}}

	unknown := uuid.New()

	testCases := []struct {
		name          string
		idsAttachment []uuid.UUID
		idsPrevious   []uuid.UUID
		wantLinked    []uuid.UUID
		wantErr       error
	}{
		{"attachments of the issue and its project are linked", []uuid.UUID{issueAttachment.IdAttachment, projectAttachment.IdAttachment}, nil, []uuid.UUID{issueAttachment.IdAttachment, projectAttachment.IdAttachment}, nil},
		{"attachment of another issue is rejected", []uuid.UUID{otherIssueAttachment.IdAttachment}, nil, nil, errs.ErrAttachmentLinkInvalid},
		{"attachment of another project is rejected", []uuid.UUID{otherProjectAttachment.IdAttachment}, nil, nil, errs.ErrAttachmentLinkInvalid},
		{"unknown attachment is rejected", []uuid.UUID{unknown}, nil, nil, errs.ErrAttachmentLinkInvalid},
		{"a bad link the description already had is skipped", []uuid.UUID{unknown, otherProjectAttachment.IdAttachment, issueAttachment.IdAttachment}, []uuid.UUID{unknown, otherProjectAttachment.IdAttachment}, []uuid.UUID{issueAttachment.IdAttachment}, nil},
		{"no links clears the issue links", nil, nil, []uuid.UUID{}, nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			linker := newFakeAttachmentLinker(idIssue, issueAttachment, projectAttachment, otherIssueAttachment, otherProjectAttachment)
			issue := &model.Issue{IdIssue: idIssue, IdProject: idProject, Description: attachmentLinks(testCase.idsAttachment)}

			err := service.NewAttachmentLinkService(linker).LinkIssueAttachments(context.Background(), issue, attachmentLinks(testCase.idsPrevious))

			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				require.Len(t, linker.links[idIssue], 1, "links must stay untouched when the description is rejected")
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.wantLinked, linker.links[idIssue])
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
