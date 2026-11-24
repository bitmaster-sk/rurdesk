package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestIssueReadState_PerReader(t *testing.T) {
	app := Setup(t)
	ownerToken := Token(t, app)
	idProject := createProject(t, app, ownerToken, "issue-read")
	readerToken := registerMemberInProject(t, app, ownerToken, idProject, "issue-reader@test.sk")

	issueRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"read state","description":"body","estimated":0}`, ownerToken)
	require.Equal(t, http.StatusOK, issueRes.StatusCode)
	var createdIssue model.Issue
	require.NoError(t, json.NewDecoder(issueRes.Body).Decode(&createdIssue))

	post := func(token string, body string) int64 {
		payload := fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":4,"message":%q}`, createdIssue.IdIssue, body)
		res := Request(t, app, "POST", "/api/private/message", payload, token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var message model.Message
		require.NoError(t, json.NewDecoder(res.Body).Decode(&message))
		return message.IdMessage
	}
	readFlags := func(token string) map[int64]bool {
		res := Request(t, app, "GET",
			fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", createdIssue.IdIssue), "", token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var messages []*model.Message
		require.NoError(t, json.NewDecoder(res.Body).Decode(&messages))
		flags := map[int64]bool{}
		for _, message := range messages {
			flags[message.IdMessage] = message.IsRead
		}
		return flags
	}
	markRead := func(token string, recipientType int) int {
		res := Request(t, app, "POST", "/api/private/message/read",
			fmt.Sprintf(`{"idRecipient":%d,"idMessageRecipientType":%d}`, createdIssue.IdIssue, recipientType), token)
		return res.StatusCode
	}

	idOwnerComment := post(ownerToken, "from the owner")
	idReaderComment := post(readerToken, "from the reader")

	flags := readFlags(readerToken)
	require.False(t, flags[idOwnerComment], "another user's message is unread before any read")
	require.True(t, flags[idReaderComment], "the reader's own message is read")
	require.True(t, readFlags(ownerToken)[idOwnerComment])

	require.Equal(t, http.StatusOK, markRead(readerToken, 4))
	idLaterComment := post(ownerToken, "after the read")

	flags = readFlags(readerToken)
	require.True(t, flags[idOwnerComment], "older messages are read after marking the issue")
	require.False(t, flags[idLaterComment], "a message posted after the read is unread")
	require.False(t, readFlags(ownerToken)[idReaderComment], "read state is per reader")

	require.Equal(t, http.StatusOK, markRead(readerToken, 4))
	require.True(t, readFlags(readerToken)[idLaterComment], "a second read moves the mark forward")

	require.Equal(t, http.StatusBadRequest, markRead(readerToken, 5))

	outsiderToken := createUserAndLogin(t, app, ownerToken, "issue-read-outsider@test.sk")
	require.NotEqual(t, http.StatusOK, markRead(outsiderToken, 4), "an outsider cannot mark the issue read")
}
