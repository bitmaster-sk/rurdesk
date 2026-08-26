package common

import (
	"strings"
	"testing"
)

func skillTask(skills ...Skill) Task {
	return Task{
		IdTask:        100,
		IdRun:         42,
		IdIssue:       7,
		IdProject:     1,
		IdIssuePublic: 11,
		Branch:        "agent/b84/i7/abc",
		IssueTitle:    "Test",
		IssueDesc:     "Desc",
		Stage:         StageImplementation,
		AttemptNo:     1,
		Skills:        skills,
	}
}

func TestRenderPrompt_RendersEverySkill(t *testing.T) {
	out, err := RenderPrompt(skillTask(
		Skill{Name: "Verification rules", Content: "Run the project's checks."},
		Skill{Name: "PR rules", Content: "Write a real PR description."},
	))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}

	if !strings.Contains(out, "## Skills") {
		t.Error("prompt is missing the Skills section")
	}
	for _, want := range []string{
		"### Verification rules",
		"Run the project's checks.",
		"### PR rules",
		"Write a real PR description.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}

	// The stage body must start under its own "## Instructions" heading, not run on
	// from the last skill.
	if !strings.Contains(out, "Write a real PR description.\n\n## Instructions\n") {
		t.Errorf("stage body must be separated from the last skill by the Instructions heading:\n%s", out)
	}
	if strings.Count(out, "## Instructions") != 1 {
		t.Errorf("want exactly one Instructions heading, got %d", strings.Count(out, "## Instructions"))
	}
}

func TestRenderPrompt_KeepsInstructionsHeadingWithoutSkills(t *testing.T) {
	out, err := RenderPrompt(skillTask())
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if !strings.Contains(out, "## Instructions\n") {
		t.Error("the stage body must keep its Instructions heading when no skill applies")
	}
	if strings.Contains(out, "## Instructions\n\n## Instructions") {
		t.Error("Instructions heading rendered twice")
	}
}

func TestRenderPrompt_OmitsSectionWithoutSkills(t *testing.T) {
	out, err := RenderPrompt(skillTask())
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if strings.Contains(out, "## Skills") {
		t.Error("a task with no skills must not render an empty Skills section")
	}
}

func TestParseStageExecute_MapsSkillsAndSkipsMalformed(t *testing.T) {
	payload := map[string]any{
		"idRun":     float64(42),
		"idIssue":   float64(7),
		"idProject": float64(1),
		"idUserBot": float64(3),
		"payload": map[string]any{
			"idTask":    float64(100),
			"stage":     StageImplementation,
			"attemptNo": float64(1),
			"contextBundle": map[string]any{
				"skills": []any{
					map[string]any{"name": "Verification rules", "content": "Run the checks."},
					map[string]any{"name": "", "content": "no name"},
					map[string]any{"name": "no content", "content": ""},
					"not an object",
				},
			},
		},
	}

	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("parseStageExecutePayload error: %v", err)
	}

	if len(task.Skills) != 1 {
		t.Fatalf("want 1 usable skill, got %d", len(task.Skills))
	}
	if task.Skills[0].Name != "Verification rules" || task.Skills[0].Content != "Run the checks." {
		t.Errorf("skill mapped wrong: %+v", task.Skills[0])
	}
}

func TestParseStageExecute_NoSkillsFieldIsFine(t *testing.T) {
	payload := map[string]any{
		"idRun":     float64(42),
		"idIssue":   float64(7),
		"idProject": float64(1),
		"idUserBot": float64(3),
		"payload": map[string]any{
			"idTask":        float64(100),
			"stage":         StageDesign,
			"contextBundle": map[string]any{},
		},
	}

	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("parseStageExecutePayload error: %v", err)
	}

	if len(task.Skills) != 0 {
		t.Errorf("an older tracker sends no skills field; got %d skills", len(task.Skills))
	}
}
