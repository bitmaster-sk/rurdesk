package goose

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const guardHooksDir = "plugins/rurdesk-guard/hooks"

func runGuardHook(t *testing.T, script, stageMarker string) string {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(guardHooksDir, script))
	cmd.Env = append(os.Environ(), stageMarkerEnv+"="+stageMarker)
	cmd.Stdin = strings.NewReader(`{"event":"Stop","session_id":"s1"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s failed: %v", script, err)
	}
	return string(out)
}

func TestGuardStopHook(t *testing.T) {
	t.Run("blocks the turn before the stage is submitted", func(t *testing.T) {
		out := runGuardHook(t, "require-submission.sh", filepath.Join(t.TempDir(), "marker"))

		var decision struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if err := json.Unmarshal([]byte(out), &decision); err != nil {
			t.Fatalf("hook output is not JSON: %v\n%s", err, out)
		}
		if decision.Decision != "block" || !strings.Contains(decision.Reason, "tracker__complete_stage") {
			t.Errorf("decision = %+v, want a block that names tracker__complete_stage", decision)
		}
	})
	t.Run("lets the turn end once the stage is submitted", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "marker")
		runGuardHook(t, "mark-submitted.sh", marker)

		if out := runGuardHook(t, "require-submission.sh", marker); out != "" {
			t.Errorf("hook output = %q, want none", out)
		}
	})
	t.Run("stays out of runs the gateway did not mark", func(t *testing.T) {
		if out := runGuardHook(t, "require-submission.sh", ""); out != "" {
			t.Errorf("hook output = %q, want none", out)
		}
	})
}

func TestGuardHooksMatchCompleteStage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(guardHooksDir, "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooksFile struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("hooks.json: %v", err)
	}

	postToolUse := hooksFile.Hooks["PostToolUse"]
	if len(postToolUse) != 1 || !strings.Contains(postToolUse[0].Hooks[0].Command, "mark-submitted.sh") {
		t.Fatalf("PostToolUse = %+v, want one rule running mark-submitted.sh", postToolUse)
	}
	matcher := regexp.MustCompile(postToolUse[0].Matcher)
	for tool, want := range map[string]bool{
		"tracker__complete_stage": true,
		"shell":                   false,
		"tracker__get_issue":      false,
	} {
		if got := matcher.MatchString(tool); got != want {
			t.Errorf("matcher %q on %q = %v, want %v", postToolUse[0].Matcher, tool, got, want)
		}
	}

	stop := hooksFile.Hooks["Stop"]
	if len(stop) != 1 || !strings.Contains(stop[0].Hooks[0].Command, "require-submission.sh") {
		t.Errorf("Stop = %+v, want one rule running require-submission.sh", stop)
	}
}
