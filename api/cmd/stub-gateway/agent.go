package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
)

// configuredAgent is one agent the stub answers for: its two protocol tokens and the script driving its runs.
type configuredAgent struct {
	apiKey string
	secret []byte
	script map[string]StageScript
}

// agentRegistry holds every agent the stub currently answers for.
type agentRegistry struct {
	mu      sync.RWMutex
	entries []*configuredAgent
}

// put registers an agent, replacing any earlier registration of the same api key.
func (r *agentRegistry) put(target *configuredAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index, existing := range r.entries {
		if existing.apiKey == target.apiKey {
			r.entries[index] = target
			return
		}
	}
	r.entries = append(r.entries, target)
}

// bySignature identifies the addressed agent by which registered secret signs the body.
func (r *agentRegistry) bySignature(sig string, body []byte) *configuredAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.entries {
		if agent.VerifySignature(entry.secret, sig, body) == nil {
			return entry
		}
	}
	return nil
}

func (r *agentRegistry) isEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries) == 0
}

func (r *agentRegistry) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = nil
}

// handleConfigureAgent takes an agent's two tokens after the caller has created it, for a stack where the agent does not exist yet when this process starts.
func handleConfigureAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		GatewayToTrackerToken string                 `json:"gatewayToTrackerToken"`
		TrackerToGatewayToken string                 `json:"trackerToGatewayToken"`
		Script                map[string]StageScript `json:"script"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	secret, err := decodeHexSecret(body.TrackerToGatewayToken)
	if err != nil || body.GatewayToTrackerToken == "" {
		http.Error(w, "both tokens are required, the tracker one as hex", http.StatusBadRequest)
		return
	}
	agents.put(&configuredAgent{apiKey: body.GatewayToTrackerToken, secret: secret, script: body.Script})
	log.Printf("[stub-gw] credentials configured, %d scripted stage(s)", len(body.Script))
	w.WriteHeader(http.StatusNoContent)
}
