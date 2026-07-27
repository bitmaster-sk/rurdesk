package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// createScheduledIssue POSTs an issue with a scheduledAt so it lands in the Gantt
// scheduled section, and returns the decoded model (IdIssue + IdIssuePublic).
func createScheduledIssue(t *testing.T, app *issue.Application, token string, idProject int64, title string) model.Issue {
	body := fmt.Sprintf(`{"title":%q,"description":"desc","estimated":0,"scheduledAt":"2026-02-01T00:00:00Z"}`, title)
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var iss model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&iss))
	return iss
}

func ganttOrderURL(idProject int64) string {
	return fmt.Sprintf("/api/private/project/%d/gantt-order", idProject)
}
