package common

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const ProcessKillGrace = 10 * time.Second

// ProcessSession holds a running agent subprocess and a cancel hook so
// orchestrator-driven Cancel calls can stop the right child.
type ProcessSession struct {
	Cancel context.CancelFunc
}

type ProcessSessions struct {
	mu       sync.Mutex
	sessions map[RunID]*ProcessSession
}

func NewProcessSessions() *ProcessSessions {
	return &ProcessSessions{sessions: make(map[RunID]*ProcessSession)}
}

func (s *ProcessSessions) Add(runID RunID, session *ProcessSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[runID] = session
}

func (s *ProcessSessions) Remove(runID RunID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, runID)
}

func (s *ProcessSessions) take(runID RunID) (*ProcessSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[runID]
	if ok {
		delete(s.sessions, runID)
	}
	return session, ok
}

func (s *ProcessSessions) Terminate(runID RunID) {
	session, ok := s.take(runID)
	if !ok {
		return
	}
	session.Cancel()
}

// Only the goroutine that started cmd may call Wait: a second concurrent Wait blocks forever.
func StopGracefully(cmd *exec.Cmd, grace time.Duration) {
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = grace
}

func DrainStderr(reader io.Reader, maxLineBytes int, idRun int64, source string) {
	message := source + " stderr"
	skipped, err := ReadLines(reader, maxLineBytes, func(raw []byte) {
		line := strings.TrimRight(string(raw), "\r\n")
		if line == "" {
			return
		}
		log.WithLevel(stderrLineLevel(line)).Int64("idRun", idRun).Str("line", line).Msg(message)
	})
	LogReadOutcome(idRun, message, skipped, err)
}

func LogReadOutcome(idRun int64, stream string, skipped int, err error) {
	if skipped > 0 {
		log.Warn().Int64("idRun", idRun).Str("stream", stream).Int("skipped", skipped).Msg("skipped output lines over MAX_OUTPUT_LINE_MB")
	}
	if err != nil {
		log.Warn().Int64("idRun", idRun).Str("stream", stream).Err(err).Msg("reading agent output failed")
	}
}

// stderrLineLevel picks the log level by prefix, so a real failure doesn't
// read as routine debug noise.
func stderrLineLevel(line string) zerolog.Level {
	trimmed := strings.TrimLeft(line, " \t")
	switch {
	case hasPrefixFold(trimmed, "Error"),
		hasPrefixFold(trimmed, "Fatal"),
		hasPrefixFold(trimmed, "panic"):
		return zerolog.ErrorLevel
	case hasPrefixFold(trimmed, "Warning"),
		hasPrefixFold(trimmed, "Warn"),
		hasPrefixFold(trimmed, "Deprecated"):
		return zerolog.WarnLevel
	default:
		return zerolog.DebugLevel
	}
}

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}

func NumberField(fields map[string]any, key string) (int, bool) {
	switch value := fields[key].(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	case int64:
		return int(value), true
	}
	return 0, false
}
