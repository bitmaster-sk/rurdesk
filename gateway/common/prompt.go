package common

import (
	"bytes"
	"fmt"
	"text/template"
)

const promptHeader = `You are an AI agent working on issue #{{.IdIssuePublic}} in project.

## ⚠️ How this stage ends — read this first
Nothing you write reaches the tracker on its own. The ONLY way to submit your
work and finish this stage is to call the MCP tool ` + "`complete_stage`" + `. Any design,
plan, answer, or summary you write as a normal message is DISCARDED — it is NOT
your submission. If you end your turn without calling ` + "`complete_stage`" + `, all your
work is lost and the run hangs in this stage forever. Calling ` + "`complete_stage`" + ` is
mandatory and is the LAST thing you do.

## Issue
**Title:** {{.IssueTitle}}
**Description:**
{{.IssueDesc}}

## Comments
{{range .Comments}}
[{{.CreatorName}} at {{.CreatedAt}}]: {{.Message}}
{{end}}

## Identifiers (use exactly these values when calling tools)
- issue_id: {{.IdIssue}}     ← pass this to every tool that takes issue_id
- project_id: {{.IdProject}} ← pass this to every tool that takes project_id
- run_id: {{.IdRun}}          ← pass this to any tool that takes run_id
- task_id: {{.IdTask}}        ← pass this to complete_stage
- branch: {{.Branch}}

Note: #{{.IdIssuePublic}} is the human-readable label shown in the UI. Do NOT pass it to tools — tools always require the numeric issue_id above.

## Stage
{{.Stage}} (attempt #{{.AttemptNo}}{{if gt .AttemptNo 1}} — revision after user feedback{{end}})
{{if .ApprovedDesign}}
## Approved design (already accepted — build on this, do not redo it)
{{.ApprovedDesign}}
{{end}}{{if .ApprovedImplPlan}}
## Approved implementation plan (already accepted — implement exactly this)
{{.ApprovedImplPlan}}
{{end}}{{if .RejectedOutput}}
## Your previous {{.Stage}} output (REJECTED — revise it)
The user reviewed the version below and REJECTED it. Revise it according to the user feedback in the Comments section. Do not resubmit it unchanged and do not treat it as final.
{{.RejectedOutput}}
{{end}}{{if .ApprovedMockupRef}}
## Chosen mockup (user picked this variant)
The user reviewed the mockups in the approved design and chose: **{{.ApprovedMockupRef}}**. Implement that variant. Do NOT regenerate or re-post mockups — proceed with the implementation of the chosen design.
{{end}}
## Instructions
`

// completeStageReminder is appended to every prompt so the agent always ends
// by calling complete_stage, letting the server advance the run.
const completeStageReminder = `

When you are done, you MUST call the MCP tool ` + "`complete_stage`" + ` with the appropriate outcome. The server records the message and transitions the run. Without that call the run stays stuck in this stage forever. Writing your result as a normal message is NOT submitting it — only the ` + "`complete_stage`" + ` call with ` + "`message=<your result>`" + ` submits it.

Pass ` + "`id_task={{.IdTask}}`" + ` to complete_stage. Once it returns, this stage is finished: **stop immediately — end your turn, make no further tool calls, and produce no more output.**`

// ToolVocab is a per-adapter tool dictionary substituted into the prompt's
// ALLOWED / FORBIDDEN lists. Tool names belong to the harness, not the LLM —
// goose driving Claude/Google/Ollama all call goose's own tools — so each
// adapter selects its own vocab. Values carry their own backticks/qualifiers
// so a token can read e.g. "`text_editor` (view)".
type ToolVocab struct {
	Grep      string // search file contents
	ReadFile  string // read one file
	ListDir   string // list a directory
	Glob      string // find files by pattern
	WriteFile string // create/overwrite a file
	Replace   string // edit a file in place
	RunShell  string // run a (mutating) shell command
	Tracker   string // the tracker MCP tool family wildcard
}

// ToolVocabClaudeCode is the Claude Code CLI tool vocabulary. Built-ins are
// capitalised single words; MCP tools are exposed as `mcp__<server>__<tool>` and
// the tracker server is registered under the name "tracker".
var ToolVocabClaudeCode = ToolVocab{
	Grep:      "`Grep`",
	ReadFile:  "`Read`",
	ListDir:   "`Glob` (or `Bash` with `ls`)",
	Glob:      "`Glob`",
	WriteFile: "`Write`",
	Replace:   "`Edit`",
	RunShell:  "`Bash`",
	Tracker:   "`mcp__tracker__*`",
}

// ToolVocabGoose is Block Goose's vocabulary. Goose collapses read+write onto
// two multi-purpose tools — `text_editor` (view/write/str_replace) and
// `shell` — so ALLOWED/FORBIDDEN is expressed with a mode qualifier instead
// of distinct tool names. Tracker tools are namespaced `tracker__*` by the
// streamable_http extension.
var ToolVocabGoose = ToolVocab{
	Grep:      "`shell` (read-only `rg`/`grep`)",
	ReadFile:  "`text_editor` (view)",
	ListDir:   "`shell` (`ls`)",
	Glob:      "`shell` (`rg --files`)",
	WriteFile: "`text_editor` (write)",
	Replace:   "`text_editor` (str_replace)",
	RunShell:  "`shell` (mutating commands — commits, builds, installs)",
	Tracker:   "`tracker__*`",
}

const brainstormingBody = `You are in the BRAINSTORMING stage. Your job is to decide whether the issue needs clarification from the user before any design work begins. Use read-oriented tools to investigate the issue context.

ALLOWED tools: {{.Vocab.Grep}}, {{.Vocab.ReadFile}}, {{.Vocab.ListDir}}, {{.Vocab.Glob}}, and the {{.Vocab.Tracker}} tools.

FORBIDDEN in this stage: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}. No code changes are produced here.

Outcomes:
- If you have a question that materially affects the design, call ` + "`complete_stage`" + ` with ` + "`outcome=question_asked`" + `, ` + "`message=<your question>`" + `, ` + "`message_kind=brainstorming_question`" + `. That call ends your turn — stop right after it. The run goes to awaiting_input; the user's reply arrives later as a separate new run, so do NOT wait for it or keep working.
- If the issue is clear enough to proceed, call ` + "`complete_stage`" + ` with ` + "`outcome=no_action_needed`" + `, ` + "`message=No clarifications needed.`" + `, ` + "`message_kind=brainstorming_complete`" + `. The run advances to the Design stage.`

const designBody = `You are in the DESIGN stage. The brainstorming stage is complete. Your job is to write a high-level design document — explain the approach, the components involved, and the key changes. NO code, NO files modified. Use read tools to verify file paths and symbols.

ALLOWED tools: {{.Vocab.Grep}}, {{.Vocab.ReadFile}}, {{.Vocab.ListDir}}, {{.Vocab.Glob}}, and the {{.Vocab.Tracker}} tools.

FORBIDDEN: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}.

Steps:
1. Explore the codebase to verify every file/symbol you reference exists.
2. Write the design as plain markdown. Cover: problem statement, proposed approach, components/modules involved, key trade-offs.
3. Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + `, ` + "`message=<design markdown>`" + `, ` + "`message_kind=design`" + `.` + mockupGuidance

// mockupGuidance is appended only to the Design stage: how to embed an HTML
// mockup when requested. The tracker renders a ` + "```mockup" + ` block as a
// live sandboxed preview card in the issue activity feed.
const mockupGuidance = `

## HTML mockup (only when requested)
If the issue description or one of the comments explicitly asks to see a UI mockup (e.g. "create a mockup first", "I want to see some mockups"), include an interactive HTML mockup in your design message as a ` + "```mockup" + ` fenced block (optionally with a title, e.g. ` + "```mockup title=\"Login screen\"" + `). It renders as a live, sandboxed preview in the tracker and is part of the design message — NOT a code change or a file. If nobody asked for a mockup, do not add one.

Mockup blocks MUST be the LAST thing in the design message. Write all prose first, then the ` + "```mockup" + ` block(s) — nothing may follow them, no summary, no "next steps", no closing remarks. The tracker renders the approval button directly below the last mockup, so trailing text ends up detached from the design it belongs to. Multiple variants go one after another at the very end.`

const implementationPlanBody = `You are in the IMPLEMENTATION PLAN stage. The design was approved — it is included above under **Approved design**. Work from that; do not re-fetch it from the tracker.

## Your task
Write a concrete implementation plan where EVERY file you touch is shown as a real ` + "```diff" + ` fenced block in unified-diff format. The diff IS the plan: keep prose to ONE short line of intent per file; the diff carries the detail. This is mandatory — a plan that describes changes in prose, or summarizes them in tables, instead of ` + "```diff" + ` blocks is incomplete. This applies to NEW files too — never emit a plain code block for a new file; show it as a ` + "```diff" + ` against ` + "`/dev/null`" + `.

## Output format (REQUIRED)
Example — modifying an existing file:
` + "```diff" + `
--- a/path/to/file.go
+++ b/path/to/file.go
@@ ... @@
-old line
+new line
` + "```" + `

Example — creating a NEW file. You MUST include the ` + "`new file mode`" + ` header line, otherwise renderers show it as RENAMED instead of a new file:
` + "```diff" + `
diff --git a/path/to/new_file.go b/path/to/new_file.go
new file mode 100644
index 0000000..e69de29
--- /dev/null
+++ b/path/to/new_file.go
@@ -0,0 +1,3 @@
+package foo
+
+func Bar() {}
` + "```" + `

## Tools
ALLOWED: {{.Vocab.Grep}}, {{.Vocab.ReadFile}}, {{.Vocab.ListDir}}, {{.Vocab.Glob}}, and the {{.Vocab.Tracker}} tools.

FORBIDDEN: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}. Nothing is built or pushed here.

## Steps
1. Verify EVERY referenced file and symbol with {{.Vocab.Grep}} / {{.Vocab.ReadFile}} — a plan that references missing code is worse than no plan.
2. For each file: one line of intent, then a ` + "```diff" + ` block with the actual change.
3. List the tests to add, also as ` + "```diff" + ` blocks where possible.
4. Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + `, ` + "`message=<plan markdown>`" + `, ` + "`message_kind=implementation_plan`" + `.`

const implementationInstructions = `The **Approved implementation plan** above has been approved — implement exactly that; do not re-fetch it from the tracker. Implement the changes in this worktree. Commit and **push** to branch ` + "`{{.Branch}}`" + `.

Do **NOT** run ` + "`gh pr create`" + ` or open the PR/MR yourself — the tracker opens it for you from the pushed branch (it supports GitHub, GitLab and Gitea). You only push.

Once the branch is pushed, call ` + "`complete_stage`" + ` with:

- ` + "`outcome=output_submitted`" + `
- ` + "`message_kind=pull_request_pushed`" + `
- ` + "`branch_name={{.Branch}}`" + `
- ` + "`pr_title=<concise PR title>`" + `
- ` + "`pr_body=<markdown PR description of what you did>`" + `
- ` + "`message=<short summary of what you did>`" + `

Do **NOT** set ` + "`pr_url`" + ` — the tracker derives the real PR/MR URL itself.

If the user comments on the PR later, you'll be re-invoked for another Implementation attempt — push additional commits to the SAME branch ` + "`{{.Branch}}`" + ` and call ` + "`complete_stage`" + ` again. The tracker reuses the existing PR; do NOT try to open a second one.

If something goes wrong (build error you can't fix, conflicting comments, push rejected), call ` + "`complete_stage`" + ` with ` + "`outcome=errored`" + ` and ` + "`error_reason=<short reason>`" + `.`

var compiledHeader = template.Must(template.New("header").Parse(promptHeader))
var compiledReminder = template.Must(template.New("reminder").Parse(completeStageReminder))
var compiledImpl = template.Must(template.New("impl").Parse(implementationInstructions))
var compiledBrainstorming = template.Must(template.New("brainstorming").Parse(brainstormingBody))
var compiledDesign = template.Must(template.New("design").Parse(designBody))
var compiledImplPlan = template.Must(template.New("implPlan").Parse(implementationPlanBody))

// RenderPrompt renders the per-stage prompt for the agent, ending with the
// complete_stage reminder. Stage bodies substitute task.Vocab into their
// ALLOWED/FORBIDDEN lists; an unset Vocab falls back to Claude Code.
func RenderPrompt(task Task) (string, error) {
	if task.Vocab == (ToolVocab{}) {
		task.Vocab = ToolVocabClaudeCode
	}

	var buf bytes.Buffer
	if err := compiledHeader.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering header: %w", err)
	}

	var body *template.Template
	switch task.Stage {
	case StageBrainstorming:
		body = compiledBrainstorming
	case StageDesign:
		body = compiledDesign
	case StageImplementationPlan:
		body = compiledImplPlan
	case StageImplementation:
		body = compiledImpl
	default:
		return "", fmt.Errorf("unknown stage: %q", task.Stage)
	}
	if err := body.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering %s body: %w", task.Stage, err)
	}

	if err := compiledReminder.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering completion reminder: %w", err)
	}
	return buf.String(), nil
}
