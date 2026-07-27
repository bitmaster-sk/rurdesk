package common

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const statePath = "/var/lib/gateway/state.json"

type stateData struct {
	RunDurations []time.Duration `json:"runDurations"`
}

// State persists gateway state across restarts.
type State struct {
	mu   sync.Mutex
	data stateData
}

func NewState() *State {
	s := &State{}
	s.load()
	return s
}

func (s *State) load() {
	data, err := os.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("failed to load state")
		}
		return
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		log.Warn().Err(err).Msg("failed to parse state file")
	}
}

func (s *State) save() {
	data, err := json.Marshal(s.data)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal state")
		return
	}
	if err := os.MkdirAll("/var/lib/gateway", 0o755); err != nil {
		log.Error().Err(err).Msg("failed to create state dir")
		return
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		log.Error().Err(err).Msg("failed to write state")
	}
}

// RecordRunDuration appends a finished-run duration to the rolling window
// persisted on disk. Unread today; kept so an ETA/pacing feature can use it
// without re-introducing the schema.
func (s *State) RecordRunDuration(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.RunDurations = append(s.data.RunDurations, d)
	if len(s.data.RunDurations) > 20 {
		s.data.RunDurations = s.data.RunDurations[len(s.data.RunDurations)-20:]
	}
	s.save()
}
