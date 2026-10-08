package common

import (
	"strings"
	"testing"
)

func wikiTask(wiki *WikiContext, skills ...Skill) Task {
	task := skillTask(skills...)
	task.Stage = StageDesign
	task.Wiki = wiki
	return task
}

func TestRenderPrompt_RendersWikiPagesAndIndex(t *testing.T) {
	out, err := RenderPrompt(wikiTask(&WikiContext{
		Pages: []WikiPage{
			{Slug: "how-we-work", Title: "How we work", Version: 3, Reason: "always", Body: "Every change has a behavior test."},
			{Slug: "shared:security", Title: "Security", Version: 7, Reason: "linked", Body: "No secrets in logs."},
		},
		Index: []WikiIndexEntry{
			{Slug: "runbook", Title: "Runbook", Summary: "how to deploy"},
			{Slug: "huge", Title: "Huge", OverLimit: true},
		},
		IndexMore: 4,
	}))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}

	for _, want := range []string{
		"## Project knowledge",
		"If the code contradicts a page, trust the code and say so in your output. Do not edit the wiki.",
		"### How we work (how-we-work · v3 · always)\nEvery change has a behavior test.",
		"### Security (shared:security · v7 · linked)\nNo secrets in logs.",
		"## Wiki index",
		"`get_wiki_page`",
		"- runbook — Runbook — how to deploy\n",
		"- huge — Huge (not loaded, over limit)\n",
		"… 4 more",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt is missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderPrompt_WikiComesBeforeSkillsAndInstructions(t *testing.T) {
	out, err := RenderPrompt(wikiTask(
		&WikiContext{Pages: []WikiPage{{Slug: "a", Title: "A", Version: 1, Reason: "always", Body: "body"}}},
		Skill{Name: "Rules", Content: "Follow them."},
	))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	stage := strings.Index(out, "## Stage")
	wiki := strings.Index(out, "## Project knowledge")
	skills := strings.Index(out, "## Skills")
	instructions := strings.Index(out, "## Instructions")
	if stage >= wiki || wiki >= skills || skills >= instructions {
		t.Errorf("want Stage < Project knowledge < Skills < Instructions, got %d %d %d %d", stage, wiki, skills, instructions)
	}
}

func TestRenderPrompt_IndexOnlyWikiStillAsksToTrustTheCode(t *testing.T) {
	out, err := RenderPrompt(wikiTask(&WikiContext{Index: []WikiIndexEntry{{Slug: "runbook", Title: "Runbook"}}}))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if !strings.Contains(out, "trust the code") {
		t.Error("the wiki instruction is missing when only the index is sent")
	}
	if !strings.Contains(out, "- runbook — Runbook\n") {
		t.Errorf("index entry without a summary is rendered wrong:\n%s", out)
	}
	if strings.Contains(out, "more, find them") {
		t.Error("no more-line when nothing was cut")
	}
}

func TestRenderPrompt_OmitsWikiSectionsWithoutWiki(t *testing.T) {
	out, err := RenderPrompt(wikiTask(nil))
	if err != nil {
		t.Fatalf("RenderPrompt error: %v", err)
	}
	if strings.Contains(out, "## Project knowledge") || strings.Contains(out, "## Wiki index") {
		t.Error("a task without wiki must not render wiki sections")
	}
}

func TestParseStageExecute_MapsWikiAndSkipsMalformed(t *testing.T) {
	task, err := parseStageExecutePayload(stagePayload(map[string]any{
		"wiki": map[string]any{
			"pages": []any{
				map[string]any{"slug": "how-we-work", "title": "How we work", "version": float64(3), "reason": "always", "body": "tests first"},
				map[string]any{"slug": "", "body": "no slug"},
				map[string]any{"slug": "empty", "body": ""},
				"not an object",
			},
			"index": []any{
				map[string]any{"slug": "runbook", "title": "Runbook", "summary": "deploy", "overLimit": true},
				map[string]any{"title": "no slug"},
			},
			"indexMore": float64(2),
		},
	}))
	if err != nil {
		t.Fatalf("parseStageExecutePayload error: %v", err)
	}
	if task.Wiki == nil {
		t.Fatal("wiki was not mapped")
	}
	if len(task.Wiki.Pages) != 1 || task.Wiki.Pages[0] != (WikiPage{Slug: "how-we-work", Title: "How we work", Version: 3, Reason: "always", Body: "tests first"}) {
		t.Errorf("pages mapped wrong: %+v", task.Wiki.Pages)
	}
	if len(task.Wiki.Index) != 1 || task.Wiki.Index[0] != (WikiIndexEntry{Slug: "runbook", Title: "Runbook", Summary: "deploy", OverLimit: true}) {
		t.Errorf("index mapped wrong: %+v", task.Wiki.Index)
	}
	if task.Wiki.IndexMore != 2 {
		t.Errorf("indexMore = %d, want 2", task.Wiki.IndexMore)
	}
}

func TestParseStageExecute_MissingNullOrEmptyWikiIsNil(t *testing.T) {
	for name, bundle := range map[string]map[string]any{
		"older tracker": {},
		"null":          {"wiki": nil},
		"empty":         {"wiki": map[string]any{"pages": []any{}, "index": []any{}, "indexMore": float64(0)}},
	} {
		task, err := parseStageExecutePayload(stagePayload(bundle))
		if err != nil {
			t.Fatalf("%s: parseStageExecutePayload error: %v", name, err)
		}
		if task.Wiki != nil {
			t.Errorf("%s: want no wiki, got %+v", name, task.Wiki)
		}
	}
}

func stagePayload(bundle map[string]any) map[string]any {
	return map[string]any{
		"idRun":       float64(42),
		"idIssue":     float64(7),
		"idProject":   float64(1),
		"idUserAgent": float64(3),
		"payload": map[string]any{
			"idTask":        float64(100),
			"stage":         StageDesign,
			"attemptNo":     float64(1),
			"contextBundle": bundle,
		},
	}
}
