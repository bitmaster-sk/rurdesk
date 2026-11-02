package mcp

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCompleteStage_MessageKindSchemaListsTheAgentKinds(t *testing.T) {
	server := buildMCPServer(NewDispatcher(gin.New()), StageAll)
	tool := server.GetTool("complete_stage")
	require.NotNil(t, tool)

	property, ok := tool.Tool.InputSchema.Properties["message_kind"].(map[string]any)
	require.True(t, ok)
	require.ElementsMatch(t, constants.AgentMessageKinds, property["enum"])
	require.NotContains(t, property["enum"], string(constants.MessageKindComment))
}

func TestPlanServer_StillCannotProposeWikiChanges(t *testing.T) {
	server := buildMCPServer(NewDispatcher(gin.New()), StagePlan)
	require.Nil(t, server.GetTool("suggest_wiki_change"))
}
