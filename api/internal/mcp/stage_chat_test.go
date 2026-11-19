package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// listChatTools talks JSON-RPC to /mcp/chat/http (initialize, then tools/list)
// and returns the advertised tool names.
func listChatTools(t *testing.T) map[string]bool {
	t.Helper()
	handler := NewMCPServer(&Config{}, gin.New()).Handler()

	post := func(body, session string) (*httptest.ResponseRecorder, string) {
		req := httptest.NewRequest(http.MethodPost, "/mcp/chat/http", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		out, err := io.ReadAll(w.Body)
		require.NoError(t, err)
		return w, string(out)
	}

	w, _ := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`, "")
	require.Equal(t, http.StatusOK, w.Code)
	session := w.Header().Get("Mcp-Session-Id")

	w, out := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, session)
	require.Equal(t, http.StatusOK, w.Code)

	// The response may be plain JSON or one SSE "data:" frame.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "data:")
		var resp struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &resp) == nil && len(resp.Result.Tools) > 0 {
			names := map[string]bool{}
			for _, tool := range resp.Result.Tools {
				names[tool.Name] = true
			}
			return names
		}
	}
	t.Fatalf("no tools in response: %s", out)
	return nil
}

func TestChatToolSet_ReadOnly(t *testing.T) {
	names := listChatTools(t)

	for _, want := range []string{"complete_stage", "get_issue", "search_wiki", "list_issue_messages"} {
		require.True(t, names[want], "chat must expose %s", want)
	}
	for _, banned := range []string{
		"create_issue", "update_issue", "bulk_update_issues",
		"post_issue_message", "update_issue_message",
		"create_tag", "update_tag", "delete_tag",
		"add_relation", "update_relation", "remove_relation",
		"create_track", "update_track", "delete_track",
		"suggest_wiki_change", "upsert_wiki_page",
		// The run token is refused by REST for these routes.
		"get_my_issues", "list_track_records",
	} {
		require.False(t, names[banned], "chat must not expose %s", banned)
	}
}

func TestChatToolSet_IsSubsetOfPlanPlusNothingNew(t *testing.T) {
	plan := buildMCPServer(NewDispatcher(gin.New()), StagePlan)
	chat := buildMCPServer(NewDispatcher(gin.New()), StageChat)
	for name := range chat.ListTools() {
		require.NotNil(t, plan.GetTool(name), "chat tool %s is not in the plan set", name)
	}
}

func TestChatCompleteStage_OffersChatMessageKindsOnly(t *testing.T) {
	server := buildMCPServer(NewDispatcher(gin.New()), StageChat)
	tool := server.GetTool("complete_stage")
	require.NotNil(t, tool)

	property, ok := tool.Tool.InputSchema.Properties["message_kind"].(map[string]any)
	require.True(t, ok)
	require.ElementsMatch(t, constants.ChatMessageKinds, property["enum"])
	require.NotContains(t, tool.Tool.InputSchema.Properties, "branch_name", "a chat turn opens no PR")
}

func TestAllowsWrite_ChatNever(t *testing.T) {
	require.False(t, allowsWrite(StageChat))
}
