package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type fakeIssueMessageStore struct {
	messages     []*model.Message
	parent       *repository.AnchorParent
	parentErr    error
	rootAnchor   *model.MessageAnchor
	rootIssueId  int64
	rootErr      error
	storedAnchor *model.MessageAnchor
}

func (f *fakeIssueMessageStore) LoadIssueMessages(_ context.Context, _ int64, _ int64, _ *bool) ([]*model.Message, error) {
	return f.messages, nil
}

func (f *fakeIssueMessageStore) InsertIssueMessage(_ context.Context, message string, creator *model.User, idRecipient int64) (*model.Message, error) {
	return &model.Message{
		IdMessage:              99,
		Message:                message,
		Creator:                creator,
		IdRecipient:            idRecipient,
		IdMessageRecipientType: model.IssueRecipientType,
		IsRead:                 true,
	}, nil
}

func (f *fakeIssueMessageStore) LoadAnchorParentForShare(_ context.Context, _ int64) (*repository.AnchorParent, error) {
	if f.parentErr != nil {
		return nil, f.parentErr
	}
	return f.parent, nil
}

func (f *fakeIssueMessageStore) LoadThreadRootAnchor(_ context.Context, _ int64) (*model.MessageAnchor, int64, error) {
	if f.rootErr != nil {
		return nil, 0, f.rootErr
	}
	return f.rootAnchor, f.rootIssueId, nil
}

func (f *fakeIssueMessageStore) InsertMessageAnchor(_ context.Context, _ int64, anchor *model.MessageAnchor) error {
	copyAnchor := *anchor
	f.storedAnchor = &copyAnchor
	return nil
}

func TestMessageAnchorService_LoadIssueMessagesRelocatesAndCutsAnchorText(t *testing.T) {
	longLine := strings.Repeat("a", 600)
	store := &fakeIssueMessageStore{
		messages: []*model.Message{
			{IdMessage: 1, Message: "inserted\n" + longLine, Version: 2},
			{
				IdMessage: 2,
				Anchor: &model.MessageAnchor{
					IdParentMessage: 1,
					ParentVersion:   1,
					AnchorLineStart: 1,
					AnchorLineEnd:   1,
					AnchorText:      longLine,
				},
			},
			{
				IdMessage: 3,
				Anchor: &model.MessageAnchor{
					IdParentMessage: 1,
					ParentVersion:   1,
					AnchorLineStart: 1,
					AnchorLineEnd:   1,
					AnchorText:      "removed line",
				},
			},
		},
	}

	svc := NewMessageAnchorService(store)
	msgs, err := svc.LoadIssueMessages(context.Background(), 10, 1)
	require.NoError(t, err)

	require.False(t, msgs[1].Anchor.IsOutdated)
	require.Equal(t, 2, msgs[1].Anchor.AnchorLineStart)
	require.Equal(t, 2, msgs[1].Anchor.AnchorLineEnd)
	require.Equal(t, strings.Repeat("a", anchorTextResponseLength)+"…", msgs[1].Anchor.AnchorText)

	require.True(t, msgs[2].Anchor.IsOutdated)
	require.Equal(t, "removed line", msgs[2].Anchor.AnchorText)
}

func TestMessageAnchorService_CreateIssueMessageValidatesAnchor(t *testing.T) {
	ctx := context.Background()
	creator := &model.User{IdUser: 1}

	t.Run("rejects missing parent", func(t *testing.T) {
		svc := NewMessageAnchorService(&fakeIssueMessageStore{parentErr: pgx.ErrNoRows})
		_, err := svc.CreateIssueMessage(ctx, "reply", creator, 10, &model.MessageAnchor{
			IdParentMessage: 1, AnchorLineStart: 1, AnchorLineEnd: 1,
		})
		require.ErrorIs(t, err, ErrAnchorWrongThread)
	})

	t.Run("rejects nested anchor", func(t *testing.T) {
		svc := NewMessageAnchorService(&fakeIssueMessageStore{
			parent: &repository.AnchorParent{IdIssue: 10, Body: "line 1", Version: 1, IsAnchored: true},
		})
		_, err := svc.CreateIssueMessage(ctx, "reply", creator, 10, &model.MessageAnchor{
			IdParentMessage: 1, AnchorLineStart: 1, AnchorLineEnd: 1,
		})
		require.ErrorIs(t, err, ErrAnchorNestedParent)
	})

	t.Run("rejects line out of range", func(t *testing.T) {
		svc := NewMessageAnchorService(&fakeIssueMessageStore{
			parent: &repository.AnchorParent{IdIssue: 10, Body: "line 1\nline 2\n", Version: 1},
		})
		_, err := svc.CreateIssueMessage(ctx, "reply", creator, 10, &model.MessageAnchor{
			IdParentMessage: 1, AnchorLineStart: 1, AnchorLineEnd: 3,
		})
		require.ErrorIs(t, err, ErrAnchorLineOutOfRange)
	})

	t.Run("stores full anchor text and cuts response", func(t *testing.T) {
		longLine := strings.Repeat("b", 600)
		store := &fakeIssueMessageStore{
			parent: &repository.AnchorParent{IdIssue: 10, Body: "head\r\n" + longLine + "\r\n", Version: 3},
		}
		svc := NewMessageAnchorService(store)
		msg, err := svc.CreateIssueMessage(ctx, "reply", creator, 10, &model.MessageAnchor{
			IdParentMessage: 1, AnchorLineStart: 2, AnchorLineEnd: 2,
		})
		require.NoError(t, err)
		require.Equal(t, longLine, store.storedAnchor.AnchorText)
		require.Equal(t, 3, store.storedAnchor.ParentVersion)
		require.Equal(t, strings.Repeat("b", anchorTextResponseLength)+"…", msg.Anchor.AnchorText)
	})
}
