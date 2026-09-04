package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/suite"
)

type MessageAnchorSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	IdProject int64
	IdIssue   int64
}

func (s *MessageAnchorSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"anchor-test-project","color":"#aabbcc"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"anchor test issue","description":"anchor test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issRes.Body).Decode(&iss)
	s.IdIssue = iss.IdIssue
}

func (s *MessageAnchorSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *MessageAnchorSuite) postMessage(body string) *http.Response {
	return Request(s.T(), s.App, "POST", "/api/private/message", body, s.Token)
}

func (s *MessageAnchorSuite) listMessages() []*model.Message {
	path := fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", s.IdIssue)
	res := Request(s.T(), s.App, "GET", path, "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var msgs []*model.Message
	json.NewDecoder(res.Body).Decode(&msgs)
	return msgs
}

func (s *MessageAnchorSuite) Test_01_PostAnchoredMessage_HappyPath() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"line 1\nline 2\nline 3"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)
	s.Greater(parent.IdMessage, int64(0))
	s.Equal(1, parent.Version)

	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"anchored reply","idParentMessage":%d,"anchorLineStart":2,"anchorLineEnd":2}`,
		s.IdIssue, parent.IdMessage)
	childRes := s.postMessage(childBody)
	s.Equal(http.StatusOK, childRes.StatusCode)

	var child model.Message
	json.NewDecoder(childRes.Body).Decode(&child)
	s.Require().NotNil(child.Anchor)
	s.Equal(parent.IdMessage, child.Anchor.IdParentMessage)
	s.Equal(2, child.Anchor.AnchorLineStart)
	s.Equal(2, child.Anchor.AnchorLineEnd)
	s.False(child.Anchor.IsOutdated)
}

func (s *MessageAnchorSuite) Test_02_PartialFieldsRejected() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"parent msg"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	// Only start provided (missing end)
	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"partial","idParentMessage":%d,"anchorLineStart":1}`,
		s.IdIssue, parent.IdMessage)
	res := s.postMessage(childBody)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *MessageAnchorSuite) Test_03_InvalidRangeRejected() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"parent msg"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	// end < start
	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"invalid range","idParentMessage":%d,"anchorLineStart":5,"anchorLineEnd":2}`,
		s.IdIssue, parent.IdMessage)
	res := s.postMessage(childBody)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *MessageAnchorSuite) Test_04_ParentInDifferentThread() {
	// Create a second issue
	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"other anchor issue","description":"anchor test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var otherIss model.Issue
	json.NewDecoder(issRes.Body).Decode(&otherIss)

	// Post parent on the other issue
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"other issue parent"}`, otherIss.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	// Try anchoring on the original issue with a parent from the other issue
	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"wrong thread","idParentMessage":%d,"anchorLineStart":1,"anchorLineEnd":1}`,
		s.IdIssue, parent.IdMessage)
	res := s.postMessage(childBody)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *MessageAnchorSuite) Test_05_EditingParentMarksAnchorOutdated() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"original text"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"anchored","idParentMessage":%d,"anchorLineStart":1,"anchorLineEnd":1}`,
		s.IdIssue, parent.IdMessage)
	childRes := s.postMessage(childBody)
	s.Require().Equal(http.StatusOK, childRes.StatusCode)
	var child model.Message
	json.NewDecoder(childRes.Body).Decode(&child)

	// Edit the parent
	editBody := `{"message":"edited text"}`
	editRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/message/%d", parent.IdMessage), editBody, s.Token)
	s.Require().Equal(http.StatusOK, editRes.StatusCode)
	var updatedParent model.Message
	json.NewDecoder(editRes.Body).Decode(&updatedParent)
	s.Equal(2, updatedParent.Version)

	// List messages and check child anchor is outdated
	msgs := s.listMessages()
	var foundChild *model.Message
	for _, m := range msgs {
		if m.IdMessage == child.IdMessage {
			foundChild = m
			break
		}
	}
	s.Require().NotNil(foundChild)
	s.Require().NotNil(foundChild.Anchor)
	s.True(foundChild.Anchor.IsOutdated)
}

func (s *MessageAnchorSuite) Test_06_DeletingParentCascadesAnchor() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"parent to delete"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"reply to be cascaded","idParentMessage":%d,"anchorLineStart":1,"anchorLineEnd":1}`,
		s.IdIssue, parent.IdMessage)
	childRes := s.postMessage(childBody)
	s.Require().Equal(http.StatusOK, childRes.StatusCode)
	var child model.Message
	json.NewDecoder(childRes.Body).Decode(&child)

	// Delete parent directly in DB (the HTTP message delete may not exist yet)
	_, err := s.App.Pool.Exec(context.Background(),
		"DELETE FROM messages.message WHERE id_message = $1", parent.IdMessage)
	s.Require().NoError(err)

	// Deleting the parent cascades to the anchor only. The reply lives in
	// messages.message, which has NO parent-message foreign key — the parent link
	// exists solely on messages.message_anchor (id_parent_message ON DELETE
	// CASCADE). So the anchor row is removed, but the reply message itself
	// survives (un-anchored).
	var count int
	err = s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM messages.message WHERE id_message = $1", child.IdMessage).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "the reply message survives — only its anchor is cascaded")

	err = s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM messages.message_anchor WHERE id_message = $1", child.IdMessage).Scan(&count)
	s.Require().NoError(err)
	s.Equal(0, count, "the anchor is removed via the id_parent_message cascade")
}

func (s *MessageAnchorSuite) Test_07_ListMessagesEmbedsVersionAndAnchor() {
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"embed test parent"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	childBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"embed test child","idParentMessage":%d,"anchorLineStart":1,"anchorLineEnd":1}`,
		s.IdIssue, parent.IdMessage)
	s.Require().Equal(http.StatusOK, s.postMessage(childBody).StatusCode)

	msgs := s.listMessages()
	s.Require().Greater(len(msgs), 0)

	var foundParent, foundChild *model.Message
	for _, m := range msgs {
		if m.IdMessage == parent.IdMessage {
			foundParent = m
		}
		if m.Anchor != nil && m.Anchor.IdParentMessage == parent.IdMessage {
			foundChild = m
		}
	}
	s.Require().NotNil(foundParent)
	s.Equal(1, foundParent.Version)
	s.Nil(foundParent.Anchor)

	s.Require().NotNil(foundChild)
	s.Equal(1, foundChild.Version)
	s.Require().NotNil(foundChild.Anchor)
	s.False(foundChild.Anchor.IsOutdated)
}

// errRow is a pgx.Row that always returns a fixed error from Scan.
type errRow struct{ err error }

func (r *errRow) Scan(_ ...any) error { return r.err }

// guardFailingTx wraps a real pgx.Tx. It delegates every method to the
// underlying transaction except QueryRow: when the SQL contains
// "messages.issue_message ism" (the anchor-guard SELECT), it returns an
// errRow with a synthetic non-ErrNoRows error so the guard path is
// exercised without a real DB outage.
type guardFailingTx struct{ pgx.Tx }

func (t *guardFailingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "messages.issue_message ism") {
		return &errRow{err: errors.New("simulated connection lost")}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

// Test_08_GuardDBErrorPropagatesAsNonWrongThread verifies that a DB error
// during the anchor-guard query (not pgx.ErrNoRows) is propagated as a
// wrapped error rather than collapsed into ErrAnchorWrongThread. Before the
// fix, any DB error — connection drop, timeout — was misreported as "anchor
// is in the wrong thread" (400). After the fix, only a missing parent row
// (ErrNoRows) yields ErrAnchorWrongThread; everything else falls through to
// the controller's 500 path.
func (s *MessageAnchorSuite) Test_08_GuardDBErrorPropagatesAsNonWrongThread() {
	// Create a valid parent message on the issue so insertMessage and the
	// issue_message INSERT succeed; only the guard SELECT is sabotaged.
	parentBody := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":"guard-fail parent"}`, s.IdIssue)
	parentRes := s.postMessage(parentBody)
	s.Require().Equal(http.StatusOK, parentRes.StatusCode)
	var parent model.Message
	json.NewDecoder(parentRes.Body).Decode(&parent)

	// Look up the seeded user so insertMessage has a valid creator.
	token := Token(s.T(), s.App)
	idUser := idOfUser(s.T(), s.App, token, "test@test.sk")
	creator := &model.User{IdUser: idUser}

	repo := repository.NewMessageRepository(s.App.Pool)
	anchor := &model.MessageAnchor{
		IdParentMessage: parent.IdMessage,
		AnchorLineStart: 1,
		AnchorLineEnd:   1,
	}

	// Begin a real transaction and wrap it so the guard QueryRow fails.
	ctx := context.Background()
	tx, err := s.App.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(ctx)

	failingTx := &guardFailingTx{Tx: tx}
	txCtx := extctx.WithTx(ctx, failingTx)

	_, err = repo.InsertIssueMessage(txCtx, "guard-fail child", creator, s.IdIssue, anchor)

	// The error must NOT be ErrAnchorWrongThread — it must propagate as a
	// wrapped infrastructure error.
	s.Require().Error(err, "InsertIssueMessage should return an error")
	s.False(errors.Is(err, repository.ErrAnchorWrongThread),
		"DB infrastructure error must not be collapsed to ErrAnchorWrongThread, got: %v", err)
}

func Test_RunMessageAnchorSuite(t *testing.T) {
	suite.Run(t, new(MessageAnchorSuite))
}
