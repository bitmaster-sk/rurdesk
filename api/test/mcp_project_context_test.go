package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type McpProjectContextSuite struct {
	suite.Suite
	App          *issue.Application
	OwnerToken   string
	OwnerID      int64
	MemberToken  string
	MemberID     int64
	OutsiderTok  string
	ProjectID    int64
	McpSessionID string
}

func (s *McpProjectContextSuite) SetupSuite() {
	s.App = Setup(s.T())
	admin := Token(s.T(), s.App)

	s.OwnerToken = createUserAsAdmin(s.T(), s.App, admin,
		`{"name":"mcpctx owner","email":"mcpctxowner@test.sk","password":"kreslo"}`)
	s.MemberToken = createUserAsAdmin(s.T(), s.App, admin,
		`{"name":"mcpctx member","email":"mcpctxmember@test.sk","password":"kreslo"}`)
	s.OutsiderTok = createUserAsAdmin(s.T(), s.App, admin,
		`{"name":"mcpctx outsider","email":"mcpctxoutsider@test.sk","password":"kreslo"}`)

	s.OwnerID = idOfUser(s.T(), s.App, admin, "mcpctxowner@test.sk")
	s.MemberID = idOfUser(s.T(), s.App, admin, "mcpctxmember@test.sk")

	s.ProjectID = createProject(s.T(), s.App, s.OwnerToken, "mcp-context-project")

	add := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"%s"}`, s.MemberID, model.RoleMember), s.OwnerToken)
	s.Require().Equal(http.StatusOK, add.StatusCode)

	s.McpSessionID = s.mcpInitialize()
}

func (s *McpProjectContextSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE name = 'mcp-context-project'")
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('mcpctxowner@test.sk','mcpctxmember@test.sk','mcpctxoutsider@test.sk')")
}

// mcpInitialize performs the JSON-RPC handshake and returns the session id the
// streamable-HTTP transport requires on every subsequent call.
func (s *McpProjectContextSuite) mcpInitialize() string {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{` +
		`"protocolVersion":"2024-11-05","capabilities":{},` +
		`"clientInfo":{"name":"integration-test","version":"1.0.0"}}}`
	res := RequestWithHeaders(s.T(), s.App, "POST", "/mcp/http", body, map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
		"Authorization": "Bearer " + s.OwnerToken,
	})
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	sessionID := res.Header.Get("Mcp-Session-Id")
	s.Require().NotEmpty(sessionID)
	return sessionID
}

type mcpToolResult struct {
	Result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *McpProjectContextSuite) callGetProjectContext(token string) mcpToolResult {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+
		`{"name":"get_project_context","arguments":{"project_id":%d}}}`, s.ProjectID)
	res := RequestWithHeaders(s.T(), s.App, "POST", "/mcp/http", body, map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "application/json, text/event-stream",
		"Authorization":  "Bearer " + token,
		"Mcp-Session-Id": s.McpSessionID,
	})
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var out mcpToolResult
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&out))
	s.Require().Nil(out.Error)
	s.Require().Len(out.Result.Content, 1)
	return out
}

// A project member who is not the owner is the tool's main consumer (bots can
// never be project owners), so the aggregate must succeed for them.
func (s *McpProjectContextSuite) Test_01_Member_GetsContextWithMembers() {
	out := s.callGetProjectContext(s.MemberToken)
	s.False(out.Result.IsError, out.Result.Content[0].Text)

	var ctxPayload struct {
		Project struct {
			IdProject int64 `json:"idProject"`
		} `json:"project"`
		Members []struct {
			IdUser int64  `json:"idUser"`
			Name   string `json:"name"`
		} `json:"members"`
	}
	s.Require().NoError(json.Unmarshal([]byte(out.Result.Content[0].Text), &ctxPayload))
	s.Equal(s.ProjectID, ctxPayload.Project.IdProject)

	ids := make([]int64, 0, len(ctxPayload.Members))
	for _, m := range ctxPayload.Members {
		ids = append(ids, m.IdUser)
	}
	s.Contains(ids, s.OwnerID)
	s.Contains(ids, s.MemberID)
}

func (s *McpProjectContextSuite) Test_02_Owner_GetsContextWithMembers() {
	out := s.callGetProjectContext(s.OwnerToken)
	s.False(out.Result.IsError, out.Result.Content[0].Text)
	s.Contains(out.Result.Content[0].Text, `"members"`)
}

func (s *McpProjectContextSuite) Test_03_NonMember_IsRefused() {
	out := s.callGetProjectContext(s.OutsiderTok)
	s.True(out.Result.IsError)
	s.Contains(out.Result.Content[0].Text, "forbidden")
}

func TestMcpProjectContextSuite(t *testing.T) {
	suite.Run(t, new(McpProjectContextSuite))
}
