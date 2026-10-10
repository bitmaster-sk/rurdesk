package common

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestStderrLineLevel(t *testing.T) {
	tests := []struct {
		name string
		line string
		want zerolog.Level
	}{
		{"error prefix", "Error: boom", zerolog.ErrorLevel},
		{"fatal prefix any case", "FATAL crash", zerolog.ErrorLevel},
		{"panic with leading indent", "  \tpanic: nil map", zerolog.ErrorLevel},
		{"warning prefix", "Warning: deprecated flag", zerolog.WarnLevel},
		{"warn prefix lowercase", "warn something", zerolog.WarnLevel},
		{"deprecated prefix", "Deprecated: use --x", zerolog.WarnLevel},
		{"plain line", "loading extensions", zerolog.DebugLevel},
		{"error word not at start", "no Error here", zerolog.DebugLevel},
		{"shorter than prefix", "Err", zerolog.DebugLevel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stderrLineLevel(tt.line); got != tt.want {
				t.Errorf("stderrLineLevel(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestDrainStderrLogsNonEmptyLinesWithSourceAndLevel(t *testing.T) {
	var buffer bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buffer).Level(zerolog.DebugLevel)
	t.Cleanup(func() { log.Logger = previous })

	DrainStderr(strings.NewReader("Error: boom\r\n\nloading\n"), 42, "goose")

	var entries []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			t.Fatalf("log line %q is not JSON: %v", raw, err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d log entries, want 2: %s", len(entries), buffer.String())
	}
	want := []struct{ level, line string }{{"error", "Error: boom"}, {"debug", "loading"}}
	for index, expected := range want {
		entry := entries[index]
		if entry["level"] != expected.level || entry["line"] != expected.line ||
			entry["message"] != "goose stderr" || entry["idRun"] != float64(42) {
			t.Errorf("entry %d = %v, want level=%s line=%q message=%q idRun=42",
				index, entry, expected.level, expected.line, "goose stderr")
		}
	}
}

func TestNumberField(t *testing.T) {
	fields := map[string]any{"float": float64(12), "int": 7, "int64": int64(9), "text": "5"}
	tests := []struct {
		key    string
		want   int
		wantOk bool
	}{
		{"float", 12, true},
		{"int", 7, true},
		{"int64", 9, true},
		{"text", 0, false},
		{"missing", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := NumberField(fields, tt.key)
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("NumberField(%q) = (%d, %v), want (%d, %v)", tt.key, got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestProcessSessionsTerminateStopsRunningChild(t *testing.T) {
	sessions := NewProcessSessions()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	sessions.Add("1", &ProcessSession{Cmd: cmd, Cancel: cancel})

	start := time.Now()
	sessions.Terminate("1")

	if ctx.Err() == nil {
		t.Error("run context was not cancelled")
	}
	if cmd.ProcessState == nil || cmd.ProcessState.Success() {
		t.Errorf("child should have exited from SIGTERM, state = %v", cmd.ProcessState)
	}
	if elapsed := time.Since(start); elapsed >= processKillGrace {
		t.Errorf("Terminate took %v, the child should stop on SIGTERM before the kill grace", elapsed)
	}
}

func TestProcessSessionsTerminateIsNoOpForUnknownOrRemovedRun(t *testing.T) {
	sessions := NewProcessSessions()
	sessions.Terminate("missing")

	cancelled := false
	sessions.Add("2", &ProcessSession{Cmd: exec.Command("true"), Cancel: func() { cancelled = true }})
	sessions.Remove("2")
	sessions.Terminate("2")

	if cancelled {
		t.Error("Terminate cancelled a run that was already removed")
	}
}
