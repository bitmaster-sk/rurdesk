package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

const migrationAgentTaskMessageSeen = 20261124120000

func TestAgentTaskMessageSeenMigration_BackfillsWhatStartedTasksSaw(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	ctx := context.Background()

	idProject := createProject(t, app, token, "message-seen-migration")
	issueRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"migration issue","description":"body","estimated":0}`, token)
	require.Equal(t, http.StatusOK, issueRes.StatusCode)
	var createdIssue model.Issue
	require.NoError(t, json.NewDecoder(issueRes.Body).Decode(&createdIssue))
	postComment := func(body, createdAt string) int64 {
		payload, err := json.Marshal(map[string]any{"idRecipient": createdIssue.IdIssue, "idMessageRecipientType": 4, "message": body})
		require.NoError(t, err)
		res := Request(t, app, "POST", "/api/private/message", string(payload), token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var message model.Message
		require.NoError(t, json.NewDecoder(res.Body).Decode(&message))
		_, err = app.Pool.Exec(ctx, `UPDATE messages.message SET created_at = $2 WHERE id_message = $1`, message.IdMessage, createdAt)
		require.NoError(t, err)
		return message.IdMessage
	}
	idBefore := postComment("before the attempt", "2026-01-01 10:00:00+00")
	postComment("after the attempt started", "2026-01-01 12:00:00+00")

	db := openGooseDB(t)
	require.NoError(t, goose.DownTo(db, ".", migrationAnchoredReplies))
	t.Cleanup(func() { require.NoError(t, goose.Up(db, ".")) })

	var idRun int64
	require.NoError(t, app.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, phase, stage_plan)
		SELECT $1, update_by, $2, 'awaiting_approval', '{"stages":[]}' FROM issues.issue WHERE id_issue = $1
		RETURNING id_run`, createdIssue.IdIssue, idProject).Scan(&idRun))
	insertTask := func(stage string, startedAt *string) int64 {
		var idTask int64
		require.NoError(t, app.Pool.QueryRow(ctx, `
			INSERT INTO agent.task (id_run, id_user_agent, stage, attempt_no, status, started_at)
			SELECT $1, id_user_agent, $2, 1, 'completed', $3 FROM agent.run WHERE id_run = $1
			RETURNING id_task`, idRun, stage, startedAt).Scan(&idTask))
		return idTask
	}
	startedBetween, startedFirst := "2026-01-01 11:00:00+00", "2026-01-01 09:00:00+00"
	idTaskBetween := insertTask("design", &startedBetween)
	idTaskFirst := insertTask("brainstorming", &startedFirst)
	idTaskNeverStarted := insertTask("pickup", nil)

	require.NoError(t, goose.UpTo(db, ".", migrationAgentTaskMessageSeen))

	seen := func(idTask int64) *int64 {
		var idMessageSeen *int64
		require.NoError(t, app.Pool.QueryRow(ctx, `SELECT id_message_seen FROM agent.task WHERE id_task = $1`, idTask).Scan(&idMessageSeen))
		return idMessageSeen
	}
	require.NotNil(t, seen(idTaskBetween))
	require.Equal(t, idBefore, *seen(idTaskBetween), "a task saw the messages written before it started")
	require.Nil(t, seen(idTaskFirst), "a task that started before any message saw none")
	require.Nil(t, seen(idTaskNeverStarted))
}
