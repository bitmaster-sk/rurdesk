// stub-gateway exercises the agent state machine without a real LLM.
// It receives stage_execute webhook events and replies via the
// complete_stage HTTP endpoint with canned outputs per stage.
package main

import (
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	trackerURL string
	agents     agentRegistry
	prStates   prStateStore
)

func main() {
	// Mirror the real gateway's contract: a single TRACKER_URL base, with the
	// fixed /api/private REST path appended here.
	trackerURL = strings.TrimRight(mustEnv("TRACKER_URL"), "/") + "/api/private"
	registerEnvAgent()

	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "9090"
	}

	http.HandleFunc("/event", handleGatewayEvent)
	http.HandleFunc("/configure", handleConfigureAgent)
	http.HandleFunc("/pr-state", handleSetPrState)
	// A self-hosted GitHub base URL puts the API under /api/v3.
	http.HandleFunc("/api/v3/repos/", handleGitHostRepos)
	http.HandleFunc("/repos/", handleGitHostRepos)
	log.Printf("stub-gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// registerEnvAgent registers the single agent whose tokens are known at boot; a stack whose agents are created later uses /configure instead.
func registerEnvAgent() {
	apiKey, secretHex := os.Getenv("GATEWAY_TO_TRACKER_TOKEN"), os.Getenv("TRACKER_TO_GATEWAY_TOKEN")
	if apiKey == "" && secretHex == "" {
		return
	}
	secret, err := hex.DecodeString(secretHex)
	if err != nil {
		log.Fatalf("invalid TRACKER_TO_GATEWAY_TOKEN: %v", err)
	}
	agents.put(&configuredAgent{apiKey: apiKey, secret: secret})
}

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return value
}
