package main

import (
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRegistry_PutReplacesSameAgentAndKeepsOthers(t *testing.T) {
	agents.reset()

	agents.put(&configuredAgent{apiKey: "agent-a", secret: []byte("secret-a")})
	agents.put(&configuredAgent{apiKey: "agent-b", secret: []byte("secret-b")})
	agents.put(&configuredAgent{apiKey: "agent-a", secret: []byte("secret-a2")})

	require.Len(t, agents.entries, 2, "reconfiguring an agent must not add a second entry for it")
	assert.Equal(t, []byte("secret-a2"), agents.entries[0].secret)
	assert.Equal(t, "agent-b", agents.entries[1].apiKey)
}

func TestAgentRegistry_BySignaturePicksTheSigningAgent(t *testing.T) {
	agents.reset()

	secretA := []byte("secret-a")
	secretB := []byte("secret-b")
	agents.put(&configuredAgent{apiKey: "agent-a", secret: secretA})
	agents.put(&configuredAgent{apiKey: "agent-b", secret: secretB})

	body := []byte(`{"idRun":1,"event":"stage_execute"}`)
	sig := agent.SignPayload(secretB, time.Now().Unix(), body)

	found := agents.bySignature(sig, body)
	require.NotNil(t, found, "a body signed by a registered agent must resolve to it")
	assert.Equal(t, "agent-b", found.apiKey)
}

func TestAgentRegistry_BySignatureRejectsUnknownSigner(t *testing.T) {
	agents.reset()
	agents.put(&configuredAgent{apiKey: "agent-a", secret: []byte("secret-a")})

	body := []byte(`{"idRun":1}`)
	sig := agent.SignPayload([]byte("some-other-secret"), time.Now().Unix(), body)

	assert.Nil(t, agents.bySignature(sig, body))
}
