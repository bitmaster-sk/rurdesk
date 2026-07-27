package common

import (
	"strings"
	"testing"
)

func TestRenderPrompt_EachStageProducesNonEmptyOutput(t *testing.T) {
	stages := []string{
		StageBrainstorming,
		StageDesign,
		StageImplementationPlan,
		StageImplementation,
	}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			task := Task{
				IdTask:        100,
				IdRun:         42,
				IdIssue:       7,
				IdProject:     1,
				IdIssuePublic: 11,
				Branch:        "agent/b84/i7/abc",
				IssueTitle:    "Test",
				IssueDesc:     "Desc",
				Stage:         stage,
				AttemptNo:     1,
			}
			out, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt(%q) error: %v", stage, err)
			}
			if out == "" {
				t.Fatalf("RenderPrompt(%q) returned empty string", stage)
			}
			if !strings.Contains(out, "complete_stage") {
				t.Errorf("prompt for %q does not mention complete_stage — model won't know how to finish", stage)
			}
			if !strings.Contains(out, "id_task=100") && !strings.Contains(out, "100") {
				t.Errorf("prompt for %q does not contain id_task value", stage)
			}
		})
	}
}

// The complete_stage mandate must appear both at the top (primacy) and the end
// (recency) — this guards against a weaker model skipping the tool call and
// just writing its answer. The top rule must also say a plain message is
// discarded, not submitted.
func TestRenderPrompt_MandatesCompleteStageAtTopAndBottom(t *testing.T) {
	stages := []string{StageBrainstorming, StageDesign, StageImplementationPlan, StageImplementation}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			out, err := RenderPrompt(Task{IdTask: 100, IdRun: 42, IdIssue: 7, Stage: stage, AttemptNo: 1})
			if err != nil {
				t.Fatalf("RenderPrompt(%q) error: %v", stage, err)
			}
			firstIdx := strings.Index(out, "complete_stage")
			lastIdx := strings.LastIndex(out, "complete_stage")
			if firstIdx == -1 || firstIdx == lastIdx {
				t.Fatalf("prompt for %q must mention complete_stage at least twice (top + bottom)", stage)
			}
			// The top mention must precede "## Instructions", proving it sits in
			// the header rule, not only in the body.
			instrIdx := strings.Index(out, "## Instructions")
			if instrIdx == -1 || firstIdx > instrIdx {
				t.Errorf("prompt for %q: complete_stage mandate is not in the top header rule", stage)
			}
			if !strings.Contains(out, "DISCARDED") {
				t.Errorf("prompt for %q: top rule must warn that a plain message is discarded, not submitted", stage)
			}
		})
	}
}

func TestRenderPrompt_ImplementationPlanRequiresDiffBlocks(t *testing.T) {
	task := Task{
		IdTask:    100,
		IdRun:     42,
		IdIssue:   7,
		IdProject: 1,
		Stage:     StageImplementationPlan,
		AttemptNo: 1,
	}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	// The agent must be told to emit ```diff fenced blocks — <app-diff-viewer>
	// renders those; other formats fall back to plain markdown.
	if !strings.Contains(out, "```diff") {
		t.Errorf("implementation_plan prompt does not instruct ```diff blocks; got:\n%s", out)
	}
	// Must be mandatory, not a soft parenthetical, or weaker models skip it.
	if !strings.Contains(out, "MUST") && !strings.Contains(out, "REQUIRED") {
		t.Errorf("implementation_plan prompt does not make diff blocks mandatory; got:\n%s", out)
	}
	// New files need the /dev/null vs b/<path> pattern too, or the agent falls
	// back to a plain ```go block that renders via markdown instead.
	if !strings.Contains(out, "/dev/null") {
		t.Errorf("implementation_plan prompt does not instruct new-file diffs against /dev/null; got:\n%s", out)
	}
	// Needs the `new file mode` header line, or diff2html infers a rename from
	// oldName(/dev/null) != newName and tags the file RENAMED instead of NEW.
	if !strings.Contains(out, "new file mode") {
		t.Errorf("implementation_plan prompt omits `new file mode` header; new files render as RENAMED. got:\n%s", out)
	}
}

func TestRenderPrompt_DesignStageOffersMockupOnRequest(t *testing.T) {
	// Only the design stage should teach the ```mockup convention — a mockup
	// only makes sense in the design output.
	withMockup := map[string]bool{
		StageBrainstorming:      false,
		StageDesign:             true,
		StageImplementationPlan: false,
		StageImplementation:     false,
	}
	for stage, want := range withMockup {
		t.Run(stage, func(t *testing.T) {
			task := Task{IdTask: 1, IdRun: 1, Stage: stage, AttemptNo: 1, Branch: "b"}
			out, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt(%q) error: %v", stage, err)
			}
			got := strings.Contains(out, "```mockup")
			if got != want {
				t.Errorf("stage %q: contains ```mockup = %v, want %v; got:\n%s", stage, got, want, out)
			}
		})
	}
}

func TestRenderPrompt_DesignStagePlacesMockupLast(t *testing.T) {
	// The tracker renders the approval button right below the last mockup, so
	// the prompt must require the mockup to end the message.
	task := Task{IdTask: 1, IdRun: 1, Stage: StageDesign, AttemptNo: 1, Branch: "b"}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt(%q) error: %v", StageDesign, err)
	}
	if !strings.Contains(out, "MUST be the LAST thing in the design message") {
		t.Errorf("design prompt does not require mockups to come last; got:\n%s", out)
	}
}

func TestRenderPrompt_NoHardTurnLimitProse(t *testing.T) {
	// The "## Budget … hard limit of N turns" prose was removed since even
	// large models ignore it; the real cap is the adapter's --max-turns flag.
	stages := []string{StageBrainstorming, StageDesign, StageImplementationPlan, StageImplementation}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			task := Task{IdTask: 1, IdRun: 1, Stage: stage, AttemptNo: 1, Branch: "b", MaxTurns: 50}
			out, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt(%q) error: %v", stage, err)
			}
			if strings.Contains(out, "hard limit") || strings.Contains(out, "## Budget") {
				t.Errorf("stage %q prompt still carries the removed budget/hard-limit prose:\n%s", stage, out)
			}
		})
	}
}

func TestRenderPrompt_VocabSubstitution(t *testing.T) {
	// Each adapter's ToolVocab replaces the ALLOWED/FORBIDDEN tool tokens, and
	// must render only names its own harness has: a token from the wrong harness
	// makes the FORBIDDEN list forbid nothing.
	stages := []string{StageBrainstorming, StageDesign, StageImplementationPlan}
	for _, stage := range stages {
		t.Run(stage+"/goose", func(t *testing.T) {
			task := Task{IdTask: 1, IdRun: 1, Stage: stage, AttemptNo: 1, Branch: "b", Vocab: ToolVocabGoose}
			out, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt error: %v", err)
			}
			for _, want := range []string{"text_editor", "shell", "tracker__*"} {
				if !strings.Contains(out, want) {
					t.Errorf("goose %q prompt missing %q:\n%s", stage, want, out)
				}
			}
			for _, bad := range []string{"`Bash`", "`Write`", "mcp__tracker__*", "run_shell_command", "mcp_tracker_*"} {
				if strings.Contains(out, bad) {
					t.Errorf("goose %q prompt leaks non-goose token %q:\n%s", stage, bad, out)
				}
			}
		})
		t.Run(stage+"/claude", func(t *testing.T) {
			task := Task{IdTask: 1, IdRun: 1, Stage: stage, AttemptNo: 1, Branch: "b", Vocab: ToolVocabClaudeCode}
			out, err := RenderPrompt(task)
			if err != nil {
				t.Fatalf("RenderPrompt error: %v", err)
			}
			for _, want := range []string{"`Bash`", "`Read`", "`Grep`", "mcp__tracker__*"} {
				if !strings.Contains(out, want) {
					t.Errorf("claude %q prompt missing %q:\n%s", stage, want, out)
				}
			}
			for _, bad := range []string{"read_file", "list_directory", "run_shell_command", "write_file", "mcp_tracker_*"} {
				if strings.Contains(out, bad) {
					t.Errorf("claude %q prompt names a tool Claude Code does not have (%q):\n%s", stage, bad, out)
				}
			}
		})
	}
}

func TestRenderPrompt_DefaultsToClaudeVocab(t *testing.T) {
	// A Task with no Vocab set must fall back to the Claude Code dictionary.
	task := Task{IdTask: 1, IdRun: 1, Stage: StageBrainstorming, AttemptNo: 1, Branch: "b"}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if !strings.Contains(out, "`Bash`") {
		t.Errorf("unset Vocab did not default to ToolVocabClaudeCode:\n%s", out)
	}
}

func TestRenderPrompt_RejectsUnknownStage(t *testing.T) {
	task := Task{IdTask: 1, Stage: "nonsense"}
	if _, err := RenderPrompt(task); err == nil {
		t.Errorf("expected error for unknown stage")
	}
}

func TestRenderPrompt_RevisionHintForAttemptN(t *testing.T) {
	task := Task{
		IdTask:    1,
		IdRun:     1,
		Stage:     StageDesign,
		AttemptNo: 3,
	}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "revision") {
		t.Errorf("attempt > 1 prompt should mention revision; got:\n%s", out)
	}
}

func TestRenderPrompt_InjectsApprovedArtifacts(t *testing.T) {
	const designMD = "DESIGN-MARKDOWN-BODY"
	const planMD = "PLAN-MARKDOWN-BODY"

	t.Run("implementation renders both above the instructions", func(t *testing.T) {
		task := Task{
			IdTask: 1, IdRun: 1, Stage: StageImplementation, AttemptNo: 1, Branch: "b",
			ApprovedDesign: designMD, ApprovedImplPlan: planMD,
		}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if !strings.Contains(out, designMD) {
			t.Errorf("prompt missing injected design body:\n%s", out)
		}
		if !strings.Contains(out, planMD) {
			t.Errorf("prompt missing injected plan body:\n%s", out)
		}
		// Artifacts must sit above the stage instructions so "the plan above" holds.
		if di, ii := strings.Index(out, planMD), strings.Index(out, "## Instructions"); di < 0 || ii < 0 || di > ii {
			t.Errorf("plan not rendered above ## Instructions (planIdx=%d instrIdx=%d)", di, ii)
		}
	})

	t.Run("empty artifacts render no headings", func(t *testing.T) {
		task := Task{IdTask: 1, IdRun: 1, Stage: StageDesign, AttemptNo: 1, Branch: "b"}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if strings.Contains(out, "Approved design") || strings.Contains(out, "Approved implementation plan") {
			t.Errorf("empty artifacts should render no headings:\n%s", out)
		}
	})

	t.Run("only design present", func(t *testing.T) {
		task := Task{
			IdTask: 1, IdRun: 1, Stage: StageImplementationPlan, AttemptNo: 1, Branch: "b",
			ApprovedDesign: designMD,
		}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if !strings.Contains(out, "Approved design") {
			t.Errorf("expected design heading; got:\n%s", out)
		}
		if strings.Contains(out, "Approved implementation plan") {
			t.Errorf("plan heading should be absent when ApprovedImplPlan is empty:\n%s", out)
		}
	})
}

func TestRenderPrompt_InjectsRejectedOutput(t *testing.T) {
	const rejectedMD = "REJECTED-DESIGN-BODY"

	t.Run("design revision renders the rejected output with a rejection warning", func(t *testing.T) {
		task := Task{
			IdTask: 1, IdRun: 1, Stage: StageDesign, AttemptNo: 2, Branch: "b",
			RejectedOutput: rejectedMD,
		}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if !strings.Contains(out, rejectedMD) {
			t.Errorf("prompt missing rejected output body:\n%s", out)
		}
		if !strings.Contains(out, "REJECTED") {
			t.Errorf("rejected output must be labeled as rejected:\n%s", out)
		}
		if strings.Contains(out, "already accepted") {
			t.Errorf("rejected output must NOT carry the approved label:\n%s", out)
		}
		// The rejected body must sit above the stage instructions.
		if ri, ii := strings.Index(out, rejectedMD), strings.Index(out, "## Instructions"); ri < 0 || ii < 0 || ri > ii {
			t.Errorf("rejected output not rendered above ## Instructions (rejIdx=%d instrIdx=%d)", ri, ii)
		}
	})

	t.Run("no rejected output renders no rejection heading", func(t *testing.T) {
		task := Task{IdTask: 1, IdRun: 1, Stage: StageDesign, AttemptNo: 1, Branch: "b"}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if strings.Contains(out, "REJECTED") {
			t.Errorf("no rejection heading expected without RejectedOutput:\n%s", out)
		}
	})
}

func TestRenderPrompt_ImplementationCarriesBranch(t *testing.T) {
	task := Task{
		IdTask:    1,
		IdRun:     1,
		Stage:     StageImplementation,
		AttemptNo: 1,
		Branch:    "agent/b84/i7/abc",
	}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "agent/b84/i7/abc") {
		t.Errorf("implementation prompt missing branch name; got:\n%s", out)
	}
}

func TestRenderPrompt_ChosenMockup(t *testing.T) {
	t.Run("renders the chosen-mockup instruction when a ref is present", func(t *testing.T) {
		task := Task{
			IdTask: 1, IdRun: 1, Stage: StageImplementationPlan, AttemptNo: 1, Branch: "b",
			ApprovedDesign: "DESIGN", ApprovedMockupRef: "Mockup B",
		}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if !strings.Contains(out, "Mockup B") {
			t.Errorf("prompt missing chosen mockup ref:\n%s", out)
		}
		if !strings.Contains(out, "Do NOT regenerate") {
			t.Errorf("prompt missing no-re-post instruction:\n%s", out)
		}
	})

	t.Run("omits the mockup section when no ref is chosen", func(t *testing.T) {
		task := Task{IdTask: 1, IdRun: 1, Stage: StageImplementationPlan, AttemptNo: 1, Branch: "b"}
		out, err := RenderPrompt(task)
		if err != nil {
			t.Fatalf("RenderPrompt error: %v", err)
		}
		if strings.Contains(out, "Chosen mockup") {
			t.Errorf("unexpected chosen-mockup section when none chosen:\n%s", out)
		}
	})
}
