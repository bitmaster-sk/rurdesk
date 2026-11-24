package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

const (
	migrationBeforeAnchorText = 20261116120000
	migrationAnchoredReplies  = 20261123120000
)

func TestMessageAnchorTextMigration_BackfillsExistingAnchors(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	ctx := context.Background()

	idProject := createProject(t, app, token, "anchor-migration")
	issueRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"migration issue","description":"body","estimated":0}`, token)
	require.Equal(t, http.StatusOK, issueRes.StatusCode)
	var createdIssue model.Issue
	require.NoError(t, json.NewDecoder(issueRes.Body).Decode(&createdIssue))

	postComment := func(body string) int64 {
		payload, err := json.Marshal(map[string]any{"idRecipient": createdIssue.IdIssue, "idMessageRecipientType": 4, "message": body})
		require.NoError(t, err)
		res := Request(t, app, "POST", "/api/private/message", string(payload), token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var message model.Message
		require.NoError(t, json.NewDecoder(res.Body).Decode(&message))
		return message.IdMessage
	}
	idParent := postComment("first\r\nsecond\r\nthird\r\n")
	idFirstReply := postComment("first reply")
	idSecondReply := postComment("second reply")
	idPastEnd := postComment("past the end")
	idEditedParent := postComment("rewritten\nsince")
	idOnOldVersion := postComment("about the old text")

	db := openGooseDB(t)
	require.NoError(t, goose.DownTo(db, ".", migrationBeforeAnchorText))
	t.Cleanup(func() { require.NoError(t, goose.Up(db, ".")) })

	_, err := app.Pool.Exec(ctx, `
		INSERT INTO messages.message_anchor (id_message, id_parent_message, parent_version, anchor_line_start, anchor_line_end)
		VALUES ($1, $4, 1, 2, 3), ($2, $4, 1, 2, 3), ($3, $4, 1, 3, 5)
	`, idFirstReply, idSecondReply, idPastEnd, idParent)
	require.NoError(t, err)
	_, err = app.Pool.Exec(ctx, `UPDATE messages.message SET version = 2 WHERE id_message = $1`, idEditedParent)
	require.NoError(t, err)
	_, err = app.Pool.Exec(ctx, `
		INSERT INTO messages.message_anchor (id_message, id_parent_message, parent_version, anchor_line_start, anchor_line_end)
		VALUES ($1, $2, 1, 1, 1)
	`, idOnOldVersion, idEditedParent)
	require.NoError(t, err)

	require.NoError(t, goose.UpTo(db, ".", migrationAnchoredReplies))

	type storedAnchor struct {
		anchorText      string
		idThreadMessage *int64
	}
	load := func(idMessage int64) storedAnchor {
		var anchor storedAnchor
		require.NoError(t, app.Pool.QueryRow(ctx,
			`SELECT anchor_text, id_thread_message FROM messages.message_anchor WHERE id_message = $1`, idMessage,
		).Scan(&anchor.anchorText, &anchor.idThreadMessage))
		return anchor
	}

	first := load(idFirstReply)
	require.Equal(t, "second\nthird", first.anchorText)
	require.Nil(t, first.idThreadMessage, "the oldest anchor on a range is the root")

	second := load(idSecondReply)
	require.Equal(t, "second\nthird", second.anchorText)
	require.NotNil(t, second.idThreadMessage)
	require.Equal(t, idFirstReply, *second.idThreadMessage)

	require.Equal(t, "", load(idPastEnd).anchorText)
	require.Equal(t, "", load(idOnOldVersion).anchorText, "the current text is not what the anchor was made on")

	require.NoError(t, goose.Up(db, "."))
	listRes := Request(t, app, "GET",
		fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", createdIssue.IdIssue), "", token)
	require.Equal(t, http.StatusOK, listRes.StatusCode)
	var listed []*model.Message
	require.NoError(t, json.NewDecoder(listRes.Body).Decode(&listed))
	outdatedReasons := map[int64]string{
		idPastEnd:      "a range past the parent's end comes back outdated",
		idOnOldVersion: "an anchor on an older parent version stays outdated",
	}
	checked := 0
	for _, message := range listed {
		if reason, isExpected := outdatedReasons[message.IdMessage]; isExpected {
			require.NotNil(t, message.Anchor)
			require.True(t, message.Anchor.IsOutdated, reason)
			checked++
		}
	}
	require.Equal(t, len(outdatedReasons), checked)
}

func openGooseDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("TEST_DATABASE_HOST"),
		envOrDefault("TEST_DATABASE_PORT", "5432"),
		envOrDefault("TEST_DATABASE_USER", "rurdesk"),
		envOrDefault("TEST_DATABASE_PASSWORD", "rurdesk"),
		os.Getenv("TEST_DATABASE_NAME"))
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}
