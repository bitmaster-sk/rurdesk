package common

import (
	"strings"
	"testing"
)

func TestRenderPrompt_ImplementationChecksTheWikiBetweenPushAndCompletion(t *testing.T) {
	out, err := RenderPrompt(skillTask())
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if !strings.Contains(out, "`suggest_wiki_change` (project_id=1,") {
		t.Errorf("implementation prompt does not ask for wiki proposals:\n%s", out)
	}
	if strings.Contains(out, "Your proposals from earlier attempts") {
		t.Error("a first attempt has no earlier proposals to list")
	}
	push := strings.Index(out, "You only push.")
	check := strings.Index(out, "## Keep the wiki true")
	complete := strings.Index(out, "## Completing the stage")
	if push < 0 || push >= check || check >= complete {
		t.Errorf("want push < wiki check < completion, got %d %d %d", push, check, complete)
	}
}

func TestRenderPrompt_ImplementationProposesAFixInsteadOfOnlyReportingIt(t *testing.T) {
	task := skillTask()
	task.Wiki = &WikiContext{Pages: []WikiPage{{Slug: "architecture", Title: "Architecture", Version: 2, Reason: "always", Body: "lazy load"}}}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if !strings.Contains(out, "trust the code and propose a fix of the page as described under **Keep the wiki true**") {
		t.Errorf("the wiki section does not point the implementation at proposals:\n%s", out)
	}
	if strings.Contains(out, "trust the code and say so in your output") {
		t.Error("the implementation must propose a fix, not only mention it")
	}
}

func TestRenderPrompt_ListsOpenProposalsForTheNextAttempt(t *testing.T) {
	task := skillTask()
	task.AttemptNo = 2
	task.WikiProposals = []WikiProposal{
		{Kind: "update", Slug: "deploy-runbook", Title: "Deploy runbook", Reason: "the branch name changed"},
		{Kind: "create", Slug: "shared:retention", Title: "Retention", Parent: "shared:ops", Reason: "new cleanup job"},
	}
	out, err := RenderPrompt(task)
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	for _, want := range []string{
		"Your proposals from earlier attempts of this run.",
		"- update deploy-runbook — Deploy runbook: the branch name changed\n",
		"- create shared:retention — Retention (under shared:ops): new cleanup job\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt is missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderPrompt_OtherStagesDoNotProposeWikiChanges(t *testing.T) {
	out, err := RenderPrompt(wikiTask(nil))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if strings.Contains(out, "suggest_wiki_change") || strings.Contains(out, "Keep the wiki true") {
		t.Error("only the implementation stage may propose wiki changes")
	}
}

func TestParseStageExecute_MapsWikiProposalsAndSkipsMalformed(t *testing.T) {
	task, err := parseStageExecutePayload(stagePayload(map[string]any{
		"wikiProposals": []any{
			map[string]any{"kind": "move", "slug": "runbook", "title": "Runbook", "parent": "ops", "reason": "grouped", "state": "open"},
			map[string]any{"kind": "update", "title": "no slug"},
			"not an object",
		},
	}))
	if err != nil {
		t.Fatalf("parseStageExecutePayload error: %v", err)
	}
	want := WikiProposal{Kind: "move", Slug: "runbook", Title: "Runbook", Parent: "ops", Reason: "grouped"}
	if len(task.WikiProposals) != 1 || task.WikiProposals[0] != want {
		t.Errorf("proposals mapped wrong: %+v", task.WikiProposals)
	}

	older, err := parseStageExecutePayload(stagePayload(map[string]any{}))
	if err != nil {
		t.Fatalf("parseStageExecutePayload error: %v", err)
	}
	if older.WikiProposals != nil {
		t.Errorf("an older tracker sends no proposals, got %+v", older.WikiProposals)
	}
}
