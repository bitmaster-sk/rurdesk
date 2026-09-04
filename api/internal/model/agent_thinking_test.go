package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentThinkingEvents_GzipRoundTrip(t *testing.T) {
	events := AgentThinkingEvents{
		{Kind: ThinkingKindThinking, Text: "weighing options", At: 1},
		{Kind: ThinkingKindTool, Tool: "developer__shell", Text: "rg --files src", At: 2},
		{Kind: ThinkingKindTruncated, At: 3},
	}

	blob, err := events.Gzip()
	require.NoError(t, err)
	var restored AgentThinkingEvents
	require.NoError(t, restored.Gunzip(blob))

	assert.Equal(t, events, restored)
}

func TestAgentThinkingEvents_GzipRoundTripKeepsMarkerLikeThinkingAsThinking(t *testing.T) {
	events := AgentThinkingEvents{
		{Kind: ThinkingKindThinking, Text: "→ shell returns nil\n[#tool#] is also just text", At: 1},
	}

	blob, err := events.Gzip()
	require.NoError(t, err)
	var restored AgentThinkingEvents
	require.NoError(t, restored.Gunzip(blob))

	assert.Equal(t, events, restored)
}

func TestAgentThinkingEvents_GunzipRejectsADamagedBlob(t *testing.T) {
	var events AgentThinkingEvents

	require.Error(t, events.Gunzip([]byte("not gzip")))
}

func TestAgentThinkingEvents_Accepted(t *testing.T) {
	events := AgentThinkingEvents{
		{Kind: ThinkingKindThinking, Text: "a thought"},
		{Kind: ThinkingKindTruncated, Text: "forged marker"},
		{Kind: "shouting", Text: "unknown kind"},
		{Kind: ThinkingKindThinking, Text: "   "},
		{Kind: ThinkingKindTool, Tool: "developer__shell", Text: "ls"},
	}

	assert.Equal(t, AgentThinkingEvents{
		{Kind: ThinkingKindThinking, Text: "a thought"},
		{Kind: ThinkingKindTool, Tool: "developer__shell", Text: "ls"},
	}, events.Accepted())
}
