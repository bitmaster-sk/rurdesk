package common

import (
	"regexp"
	"testing"
)

// uuidV4Pattern matches a canonical RFC-4122 v4 UUID: version nibble 4 and
// variant nibble in {8,9,a,b}.
var uuidV4Pattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestMCPURLForStage(t *testing.T) {
	const base = "http://issue.proxy/mcp/sse"
	tests := []struct {
		name  string
		stage string
		want  string
	}{
		{"implementation keeps full", StageImplementation, "http://issue.proxy/mcp/sse"},
		{"brainstorming uses plan subset", StageBrainstorming, "http://issue.proxy/mcp/plan/sse"},
		{"design uses plan subset", StageDesign, "http://issue.proxy/mcp/plan/sse"},
		{"implementation_plan uses plan subset", StageImplementationPlan, "http://issue.proxy/mcp/plan/sse"},
		{"pickup uses plan subset", StagePickup, "http://issue.proxy/mcp/plan/sse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MCPURLForStage(base, tt.stage); got != tt.want {
				t.Errorf("MCPURLForStage(%q) = %q, want %q", tt.stage, got, tt.want)
			}
		})
	}
}

func TestSessionUUIDIsValidUUID(t *testing.T) {
	stages := []string{
		StagePickup, StageBrainstorming, StageDesign,
		StageImplementationPlan, StageImplementation,
	}
	for _, stage := range stages {
		for _, attempt := range []int{-1, 0, 1, 9, 42} {
			got := SessionUUID(12345, stage, attempt)
			if !uuidV4Pattern.MatchString(got) {
				t.Errorf("SessionUUID(stage=%q, attempt=%d) = %q is not a valid v4 UUID",
					stage, attempt, got)
			}
		}
	}
}

func TestSessionUUIDIsStableAndUnique(t *testing.T) {
	a1 := SessionUUID(7, StageImplementation, 1)
	a1again := SessionUUID(7, StageImplementation, 1)
	if a1 != a1again {
		t.Errorf("SessionUUID not stable: %q != %q", a1, a1again)
	}

	seen := map[string]string{}
	for _, stage := range []string{StageBrainstorming, StageDesign, StageImplementationPlan, StageImplementation} {
		for attempt := 1; attempt <= 3; attempt++ {
			for _, idRun := range []int64{1, 2, 3} {
				got := SessionUUID(idRun, stage, attempt)
				key := got
				if prev, ok := seen[key]; ok {
					t.Errorf("SessionUUID collision %q for (%d,%s,%d) and %s",
						got, idRun, stage, attempt, prev)
				}
				seen[key] = stage
			}
		}
	}
}
