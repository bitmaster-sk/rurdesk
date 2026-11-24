package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
)

const anchorTextResponseLength = 500

var (
	ErrAnchorWrongThread    = errors.New("anchor parent message belongs to a different thread")
	ErrAnchorNestedParent   = errors.New("anchor parent message is itself an anchored reply")
	ErrAnchorLineOutOfRange = errors.New("anchor line range is past the end of the parent message")
	ErrAnchorInvalidThread  = errors.New("anchor thread message is not an anchored message of this issue")
)

type IssueMessageStore interface {
	LoadIssueMessages(ctx context.Context, idIssue int64, idReader int64, read *bool) ([]*model.Message, error)
	InsertIssueMessage(ctx context.Context, message string, creator *model.User, idRecipient int64) (*model.Message, error)
	LoadAnchorParentForShare(ctx context.Context, idParentMessage int64) (*repository.AnchorParent, error)
	LoadThreadRootAnchor(ctx context.Context, idThreadMessage int64) (*model.MessageAnchor, int64, error)
	InsertMessageAnchor(ctx context.Context, idMessage int64, anchor *model.MessageAnchor) error
}

type MessageAnchorService struct {
	messageStore IssueMessageStore
}

func NewMessageAnchorService(messageStore IssueMessageStore) *MessageAnchorService {
	return &MessageAnchorService{messageStore: messageStore}
}

func (s *MessageAnchorService) LoadIssueMessages(ctx context.Context, idIssue int64, idReader int64) ([]*model.Message, error) {
	msgs, err := s.messageStore.LoadIssueMessages(ctx, idIssue, idReader, nil)
	if err != nil {
		return nil, err
	}
	s.ResolveIssueAnchors(msgs)
	return msgs, nil
}

func (s *MessageAnchorService) CreateIssueMessage(
	ctx context.Context,
	message string,
	creator *model.User,
	idIssue int64,
	anchor *model.MessageAnchor,
) (*model.Message, error) {
	msg, err := s.messageStore.InsertIssueMessage(ctx, message, creator, idIssue)
	if err != nil {
		return nil, err
	}
	if anchor == nil {
		return msg, nil
	}
	stored, err := s.attachAnchor(ctx, msg.IdMessage, idIssue, anchor)
	if err != nil {
		return nil, err
	}
	msg.Anchor = stored
	return msg, nil
}

func (s *MessageAnchorService) ResolveIssueAnchors(messages []*model.Message) {
	byId := make(map[int64]*model.Message, len(messages))
	for _, message := range messages {
		byId[message.IdMessage] = message
	}
	for _, message := range messages {
		if message.Anchor == nil {
			continue
		}
		parent, isFound := byId[message.Anchor.IdParentMessage]
		if !isFound {
			message.Anchor.IsOutdated = true
		} else {
			resolveAnchor(message.Anchor, parent.Message, parent.Version)
		}
		message.Anchor.AnchorText = cutAnchorText(message.Anchor.AnchorText)
	}
}

func (s *MessageAnchorService) attachAnchor(
	ctx context.Context,
	idMessage int64,
	idIssue int64,
	request *model.MessageAnchor,
) (*model.MessageAnchor, error) {
	anchor := *request

	if request.IdThreadMessage != nil {
		root, rootIssueId, err := s.messageStore.LoadThreadRootAnchor(ctx, *request.IdThreadMessage)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrAnchorInvalidThread
			}
			return nil, fmt.Errorf("loading anchor thread root: %w", err)
		}
		if rootIssueId != idIssue {
			return nil, ErrAnchorInvalidThread
		}
		anchor = *root
	}

	parent, err := s.messageStore.LoadAnchorParentForShare(ctx, anchor.IdParentMessage)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAnchorWrongThread
		}
		return nil, fmt.Errorf("checking anchor parent thread: %w", err)
	}
	if parent.IdIssue != idIssue {
		return nil, ErrAnchorWrongThread
	}

	if anchor.IdThreadMessage == nil {
		if parent.IsAnchored {
			return nil, ErrAnchorNestedParent
		}
		lines := messageLines(parent.Body)
		if anchor.AnchorLineEnd > len(lines) {
			return nil, ErrAnchorLineOutOfRange
		}
		anchor.AnchorText = extractAnchorText(lines, anchor.AnchorLineStart, anchor.AnchorLineEnd)
		anchor.ParentVersion = parent.Version
	}

	if err := s.messageStore.InsertMessageAnchor(ctx, idMessage, &anchor); err != nil {
		return nil, err
	}

	anchor.IsOutdated = false
	resolveAnchor(&anchor, parent.Body, parent.Version)
	anchor.AnchorText = cutAnchorText(anchor.AnchorText)
	return &anchor, nil
}

// messageLines splits a message body the way the client numbers lines: CRLF becomes LF and
// one trailing LF does not start a new line.
func messageLines(body string) []string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	return strings.Split(normalized, "\n")
}

func extractAnchorText(lines []string, lineStart int, lineEnd int) string {
	return strings.Join(lines[lineStart-1:lineEnd], "\n")
}

// resolveAnchor relocates a stored anchor in the parent's current text by matching its anchor text
// nearest to the stored start. Marks the anchor outdated when no match exists.
func resolveAnchor(anchor *model.MessageAnchor, parentBody string, parentVersion int) {
	if anchor.AnchorText == "" {
		anchor.IsOutdated = true
		return
	}
	if parentVersion == anchor.ParentVersion {
		return
	}
	targetLines := strings.Split(anchor.AnchorText, "\n")
	lines := messageLines(parentBody)
	bestStart := -1
	for start := 0; start+len(targetLines) <= len(lines); start++ {
		if !linesMatch(lines[start:start+len(targetLines)], targetLines) {
			continue
		}
		if bestStart < 0 || lineDistance(start, anchor.AnchorLineStart-1) < lineDistance(bestStart, anchor.AnchorLineStart-1) {
			bestStart = start
		}
	}
	if bestStart < 0 {
		anchor.IsOutdated = true
		return
	}
	anchor.AnchorLineStart = bestStart + 1
	anchor.AnchorLineEnd = bestStart + len(targetLines)
}

func linesMatch(lines []string, expected []string) bool {
	for index := range expected {
		if lines[index] != expected[index] {
			return false
		}
	}
	return true
}

func lineDistance(from int, to int) int {
	if from > to {
		return from - to
	}
	return to - from
}

func cutAnchorText(text string) string {
	if utf8.RuneCountInString(text) <= anchorTextResponseLength {
		return text
	}
	return string([]rune(text)[:anchorTextResponseLength]) + "…"
}
