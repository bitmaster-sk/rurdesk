package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
)

// PrState is what the fake git host reports for one repository's pull request.
type PrState struct {
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

// prStateStore holds the pull request state per repository path.
type prStateStore struct {
	mu      sync.RWMutex
	entries map[string]PrState
}

func (s *prStateStore) put(repoPath string, state PrState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = map[string]PrState{}
	}
	s.entries[repoPath] = state
}

// get reports the stored state, defaulting to an open pull request for a repository nobody configured.
func (s *prStateStore) get(repoPath string) PrState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if state, ok := s.entries[repoPath]; ok {
		return state
	}
	return PrState{State: "open"}
}

func (s *prStateStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
}

// handleGitHostRepos is the fake GitHub API the merge poller reads PR status from.
func handleGitHostRepos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if strings.HasSuffix(r.URL.Path, "/reviews") {
		w.Write([]byte(`[]`))
		return
	}

	repoPath, ok := repoPathFromPullsUrl(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported path", http.StatusNotFound)
		return
	}
	if err := json.NewEncoder(w).Encode(prStates.get(repoPath)); err != nil {
		log.Printf("[stub-gw] pr status encode error: %v", err)
	}
}

// repoPathFromPullsUrl pulls "owner/repo" out of a pull request path.
func repoPathFromPullsUrl(path string) (string, bool) {
	trimmed := strings.TrimPrefix(strings.Trim(path, "/"), "api/v3/")
	segments := strings.Split(trimmed, "/")
	if len(segments) < 5 || segments[0] != "repos" || segments[3] != "pulls" {
		return "", false
	}
	return segments[1] + "/" + segments[2], true
}

// handleSetPrState sets what the fake git host reports for a repository.
func handleSetPrState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		RepoPath string `json:"repoPath"`
		State    string `json:"state"`
		Merged   bool   `json:"merged"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RepoPath == "" {
		http.Error(w, "repoPath is required", http.StatusBadRequest)
		return
	}
	prStates.put(body.RepoPath, PrState{State: body.State, Merged: body.Merged})
	log.Printf("[stub-gw] pr state for %s = %s (merged=%v)", body.RepoPath, body.State, body.Merged)
	w.WriteHeader(http.StatusNoContent)
}
