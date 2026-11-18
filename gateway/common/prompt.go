package common

import (
	"bytes"
	"fmt"
	"text/template"
)

const stageEndWarning = `## ⚠️ How this stage ends — read this first
Nothing you write reaches the tracker on its own. The ONLY way to submit your
work and finish this stage is to call the MCP tool ` + "`complete_stage`" + `. Any design,
plan, answer, or summary you write as a normal message is DISCARDED — it is NOT
your submission. If you end your turn without calling ` + "`complete_stage`" + `, all your
work is lost and the run hangs in this stage forever. Calling ` + "`complete_stage`" + ` is
mandatory and is the LAST thing you do.
`

const promptHeader = `You are an AI agent working on issue #{{.IdIssuePublic}} in project.

` + stageEndWarning + `
## Issue
**Title:** {{.IssueTitle}}
**Description:**
{{.IssueDesc}}

## Conversation (oldest first)
This is the full thread, your own submissions included. Read it in order: when
two messages conflict, the later one is what the user wants now.
{{range .Conversation}}
[{{.CreatorName}} · {{.Kind}} · {{.CreatedAt}}]: {{.Message}}
{{end}}

## Identifiers (use exactly these values when calling tools)
- issue_id: {{.IdIssue}}     ← pass this to every tool that takes issue_id
- project_id: {{.IdProject}} ← pass this to every tool that takes project_id
- run_id: {{.IdRun}}          ← pass this to any tool that takes run_id
- task_id: {{.IdTask}}        ← pass this to complete_stage
- branch: {{.Branch}}

Note: #{{.IdIssuePublic}} is the human-readable label shown in the UI. Do NOT pass it to tools — tools always require the numeric issue_id above.

## Stage
{{.Stage}} (attempt #{{.AttemptNo}}{{if .IsRevision}} — revision after user feedback{{end}}{{if .ResumedAfter}} — continuing after the previous attempt stopped{{end}})
{{if .ResumedAfter}}
## Continuing an interrupted attempt
The previous attempt of this stage stopped before it finished (reason: ` + "`{{.ResumedAfter}}`" + `).
{{- if .ResumedWorktree}} This worktree may still hold its work on branch ` + "`{{.Branch}}`" + `: uncommitted changes and local commits. Start with ` + "`git status`" + ` and ` + "`git log --oneline origin/{{.Repo.BaseBranch}}..HEAD`" + `, then continue from where it stopped. Do not start over and do not discard these changes.
{{- else if and .PrBranch (eq .Branch .PrBranch)}} This worktree was re-created from branch ` + "`{{.Branch}}`" + ` as it is on origin. Anything the previous attempt did not push is gone. Check ` + "`git log --oneline origin/{{.Repo.BaseBranch}}..HEAD`" + ` to see what is already there.
{{- else}} Its work is not available, so this worktree starts from the base branch.
{{- end}}
{{end}}{{if .ApprovedDesign}}
## Approved design (accepted when it was written — build on it, do not redo it)
Anything the conversation asked for after this was written overrides it. Do not undo such a change to bring the code back in line with this document.
{{.ApprovedDesign}}
{{end}}{{if .ApprovedImplPlan}}
## Approved implementation plan (accepted when it was written — implement it)
Same rule as the design: a later instruction in the conversation wins over this plan, and code that follows that instruction is correct even though it diverges from what is written here.
{{.ApprovedImplPlan}}
{{end}}{{if .RejectedOutput}}
## Your previous {{.Stage}} output (REJECTED — revise it)
The user reviewed the version below and REJECTED it. Revise it according to the user feedback in the conversation above. Do not resubmit it unchanged and do not treat it as final.
{{.RejectedOutput}}
{{end}}{{if .ApprovedMockupRef}}
## Chosen mockup (user picked this variant)
The user reviewed the mockups in the approved design and chose: **{{.ApprovedMockupRef}}**. Implement that variant. Do NOT regenerate or re-post mockups — proceed with the implementation of the chosen design.
{{end}}`

const releaseHeader = `You are an AI agent writing the release notes of version **{{.Release.Name}}** in project.

` + stageEndWarning + `
## Identifiers (use exactly these values when calling tools)
- project_id: {{.IdProject}} ← pass this to every tool that takes project_id
- task_id: {{.IdTask}}        ← pass this to complete_stage

## Stage
{{.Stage}} (attempt #{{.AttemptNo}})
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
// so a token can read e.g. "`shell` (`rg --files`)".
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

// ToolVocabGoose is Block Goose's vocabulary. Goose's developer extension has
// `shell`, `tree`, `write` and `edit` but no read tool, so reading goes
// through `shell` and ALLOWED/FORBIDDEN carries a mode qualifier for it.
// Tracker tools are namespaced `tracker__*` by the streamable_http extension.
var ToolVocabGoose = ToolVocab{
	Grep:      "`shell` (read-only `rg`/`grep`)",
	ReadFile:  "`shell` (read-only `cat`/`sed -n`)",
	ListDir:   "`tree`",
	Glob:      "`shell` (`rg --files`)",
	WriteFile: "`write`",
	Replace:   "`edit`",
	RunShell:  "`shell` (mutating commands — commits, builds, installs)",
	Tracker:   "`tracker__*`",
}

const brainstormingBody = `You are in the BRAINSTORMING stage. Your job is to decide whether the issue needs clarification from the user before any design work begins. Use read-oriented tools to investigate the issue context.

ALLOWED tools: {{.Vocab.Grep}}, {{.Vocab.ReadFile}}, {{.Vocab.ListDir}}, {{.Vocab.Glob}}, and the {{.Vocab.Tracker}} tools.

FORBIDDEN in this stage: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}. No code changes are produced here.

Outcomes:
- If you have a question that materially affects the design, call ` + "`complete_stage`" + ` with ` + "`outcome=question_asked`" + `, ` + "`message=<your question>`" + `, ` + "`message_kind=brainstorming_question`" + `. That call ends your turn — stop right after it. The run goes to awaiting_input; the user's reply arrives later as a separate new run, so do NOT wait for it or keep working.
- If the issue is clear enough to proceed, call ` + "`complete_stage`" + ` with ` + "`outcome=no_action_needed`" + `, ` + "`message=No clarifications needed.`" + `, ` + "`message_kind=brainstorming_complete`" + `.{{if .IsShortWorkflow}} The next stage is the implementation, so this is your last chance to ask questions.{{else}} The run advances to the Design stage.{{end}}`

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

const implementationInstructions = `{{if not .IsShortWorkflow}}Implement the **Approved implementation plan** above, together with anything the conversation has asked for since it was written; do not re-fetch it from the tracker.{{else}}Implement the task from its **Description** and the conversation above. Read the relevant code before you change it and keep the change as small as the task allows.{{end}} Work in this worktree. Commit and **push** to branch ` + "`{{.Branch}}`" + `.

Do **NOT** run ` + "`gh pr create`" + ` or open the PR/MR yourself — the tracker opens it for you from the pushed branch (it supports GitHub, GitLab and Gitea). You only push.

## Keep the wiki true
After you push and before you call ` + "`complete_stage`" + `, compare what you changed with the wiki: the pages under **Project knowledge** and every page in the **Wiki index** your change touches. For each page your change makes wrong or incomplete, and for anything new that belongs in the wiki, call ` + "`suggest_wiki_change`" + ` (project_id={{.IdProject}}, kind, slug, reason, …). An update needs the whole new body and ` + "`base_version`" + `, the version in the page header or from ` + "`get_wiki_page`" + `. You never write the wiki yourself: a person reviews the proposal once the PR is merged. Skip this step only when nothing in the wiki is affected, and name the proposals you made in your ` + "`complete_stage`" + ` message.
{{if .WikiProposals}}
Your proposals from earlier attempts of this run. Check each against the code as it is now: call ` + "`suggest_wiki_change`" + ` again for the same page to update it, or leave it when it is still right.
{{range .WikiProposals}}
- {{.Kind}} {{.Slug}} — {{.Title}}{{if .Parent}} (under {{.Parent}}){{end}}: {{.Reason}}{{end}}
{{end}}
## Completing the stage
Once the branch is pushed, call ` + "`complete_stage`" + ` with:

- ` + "`outcome=output_submitted`" + `
- ` + "`message_kind=pull_request_pushed`" + `
- ` + "`branch_name={{.Branch}}`" + `
- ` + "`pr_title=<concise PR title>`" + `
- ` + "`pr_body=<markdown PR description of what you did>`" + `
- ` + "`message=<short summary of what you did>`" + `

Do **NOT** set ` + "`pr_url`" + ` — the tracker derives the real PR/MR URL itself.

Until the PR exists there is no other valid ending, however many attempts it takes to get there: you must push code and submit ` + "`branch_name`" + `, because the tracker opens the PR from it. Submitting without a branch beforehand leaves the run with no pull request at all.

## Reviewing the open PR
Once the PR exists, every comment the user leaves re-invokes you for another attempt of this stage. The PR is already open — never open a second one; push to the SAME branch ` + "`{{.Branch}}`" + ` and the tracker reuses it. Only attempts made after the PR exists may end without code.

Change what the comment points at and nothing else. If something adjacent is unavoidable, do it and say why. If the request contradicts an earlier decision, state the consequence in a sentence and carry it out anyway — refuse only when it would genuinely break the branch (code that stops compiling, a dependency left with nothing using it, a fix that reintroduces its own bug).

Such an attempt does not have to produce code, so it can end two ways:

- **You changed something** — push to ` + "`{{.Branch}}`" + ` and complete exactly as above, with ` + "`message_kind=pull_request_pushed`" + `.
- **You did not** — the comment asked a question, or you are refusing it as described. Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + `, ` + "`message_kind=review_reply`" + `, ` + "`message=<your reply>`" + ` and NO ` + "`branch_name`" + `. Nothing is pushed, and the user's next comment brings you back. This ending is available only once the PR exists, never on an attempt that still has to open it.

If something goes wrong (build error you can't fix, push rejected), call ` + "`complete_stage`" + ` with ` + "`outcome=errored`" + ` and ` + "`error_reason=<short reason>`" + `.`

const releaseNotesBody = `You are in the RELEASE NOTES stage. Write the release notes of version **{{.Release.Name}}**. There is no repository in this stage: do not look for code.

ALLOWED tools: the {{.Vocab.Tracker}} tools.

FORBIDDEN: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}.
{{if or .Release.Sections .Release.OpenIssues}}
The release is made of exactly the tasks below{{if .Release.Tag}}, the tasks tagged ` + "`{{.Release.Tag}}`" + `{{end}}. They are your only source and the list is complete. Do not list or search the project's other tasks, and do not decide from the tracker which tasks belong to the release: a task that is not below is not in the release, whatever its state, tags or merge requests say.

## Closed tasks
Grouped by task type. The groups help you sort the tasks; they are not the sections of the release notes.
{{range .Release.Sections}}
### {{if .Name}}{{.Name}}{{else}}Other{{end}}
{{range .Issues}}- #{{.IdIssuePublic}} {{.Title}}
{{end}}{{else}}
(none)
{{end}}{{if .Release.OpenIssues}}
## Released although not closed
The team decided to release these tasks before they are closed. They belong to the release like the closed ones.
{{range .Release.OpenIssues}}- #{{.IdIssuePublic}} {{.Title}}
{{end}}{{end}}
## Steps
1. When a title does not say enough, read the task with ` + "`get_issue`" + ` (project_id={{.IdProject}}, issue_id=the number after #). Read only the tasks listed above.
2. Write the release notes as Markdown in the structure the skills of this stage give.
3. Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + ` and ` + "`message=<the whole release notes>`" + `, without ` + "`message_kind`" + `. The message is the release notes: no greeting, no summary of your work, nothing but the Markdown. It goes into the release draft, where a person reviews it before the release.

If you cannot write them, call ` + "`complete_stage`" + ` with ` + "`outcome=errored`" + ` and ` + "`error_reason=<short reason>`" + `.{{else}}
The release has no tasks, so there is nothing to write about. Do not look for tasks in the tracker. Call ` + "`complete_stage`" + ` with ` + "`outcome=errored`" + ` and ` + "`error_reason=the release has no tasks`" + `.{{end}}`

// No wiki means no section at all. The page header and index line formats are mirrored by the
// tracker's token estimate (api/internal/service/wiki_prompt.go), so change both together.
const wikiSection = `{{with .Wiki}}
## Project knowledge
Pages from the project wiki, kept by the team. Follow them. If the code contradicts a page, trust the code and {{if eq $.Stage "implementation"}}propose a fix of the page as described under **Keep the wiki true**{{else}}say so in your output{{end}}. Do not edit the wiki.
{{range .Pages}}
### {{.Title}} ({{.Slug}} · v{{.Version}} · {{.Reason}})
{{.Body}}
{{end}}{{if or .Index .IndexMore}}
## Wiki index
Other wiki pages. Read one with ` + "`get_wiki_page`" + ` (project_id, slug) when it matters for this task; search with ` + "`search_wiki`" + ` (project_id, query).
{{range .Index}}
- {{.Slug}} — {{.Title}}{{if .Summary}} — {{.Summary}}{{end}}{{if .OverLimit}} (not loaded, over limit){{end}}{{end}}
{{if .IndexMore}}… {{.IndexMore}} more, find them with ` + "`search_wiki`" + ` or ` + "`list_wiki_pages`" + `
{{end}}{{end}}{{end}}`

// No skills means no section at all.
const skillsSection = `
{{if .Skills}}## Skills
The following instructions are mandatory for this stage. Where they conflict with your general habits, the skill wins.
{{range .Skills}}
### {{.Name}}
{{.Content}}
{{end}}
{{end}}## Instructions
`

// chatHeader opens a chat turn: no issue, no stage. The read-only rules are a
// promise the tracker and the env also enforce: the run token reaches only read
// tools and the agent process holds no git token. ALLOWED leaves out
// Vocab.ListDir, whose Claude Code entry offers Bash.
const chatHeader = `You are an AI agent answering a user in a private chat in the tracker. This is not work on an issue: you only read and answer.

` + stageEndWarning + `
## Projects
You may read the tasks and the wiki of these projects with the {{.Vocab.Tracker}} tools:
{{range .Chat.Projects}}
- {{.Name}} (project_id: {{.IdProject}}){{if .IsMain}} — main project{{end}}{{else}}
- none: this chat has no project, so there are no tasks or wiki pages to read.{{end}}

## Code
{{if .Chat.WorktreeHead}}The working directory is a checkout of the main project's repository at commit ` + "`{{.Chat.WorktreeHead}}`" + ` of ` + "`{{.Repo.BaseBranch}}`" + `. The code of the other projects is not available.
{{- else}}No repository is available: the working directory is an empty scratch directory. Answer from the tasks, the wiki and the conversation.{{end}}
{{if and .Chat.HeadCommit .Chat.WorktreeHead (ne .Chat.HeadCommit .Chat.WorktreeHead)}}
The code was refreshed since your last answer: it was at commit ` + "`{{.Chat.HeadCommit}}`" + ` and is now at ` + "`{{.Chat.WorktreeHead}}`" + `. Earlier answers in the conversation may describe code that has changed since.
{{end}}
## Read-only
You only read; you must not change anything:
- no code changes: do not create, edit or delete files, and do not commit or push;
- no changes in the tracker: do not create or update tasks, comments, messages or wiki pages. The tracker refuses such calls in a chat.

ALLOWED tools: {{.Vocab.Grep}}, {{.Vocab.ReadFile}}, {{.Vocab.Glob}}, the read-only {{.Vocab.Tracker}} tools, and web search and fetch when you have them.

FORBIDDEN: {{.Vocab.WriteFile}}, {{.Vocab.Replace}}, {{.Vocab.RunShell}}.

## Conversation (oldest first)
{{if .Chat.IsHistoryTruncated}}The conversation is longer than what fits here: its oldest messages were left out. Ask the user when you miss something said earlier.
{{end}}{{range .Conversation}}
[{{.CreatorName}} · {{.Kind}} · {{.CreatedAt}}]: {{.Message}}
{{end}}
## Identifiers (use exactly these values when calling tools)
- task_id: {{.IdTask}} ← pass this to complete_stage
- run_id: {{.IdRun}}
`

const chatBody = `{{if eq .Chat.Mode "task_draft"}}{{if .Chat.DraftFromMessage}}The user asked you to draft one task about the answer quoted below; use the rest of the conversation only as context.

[Answer to draft the task from]:
{{.Chat.DraftFromMessage.Body}}

{{else}}The user asked you to draft a task from this conversation. {{end}}Write it in Markdown: the first line is ` + "`# <title>`" + ` with a short title, then a description someone can start work from — the goal, what is known from the conversation, and open questions. Do not create the task; the user reviews the draft and creates it.
Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + `, ` + "`message=<the draft>`" + `, ` + "`message_kind=task_draft`" + `.
{{- else}}Answer the user's latest message.
Call ` + "`complete_stage`" + ` with ` + "`outcome=output_submitted`" + `, ` + "`message=<your answer, plain markdown>`" + `, ` + "`message_kind=chat_reply`" + `.
If the user asks you to create or draft a task, reply with ` + "`message_kind=task_draft`" + ` and ` + "`# <title>`" + ` as the first line of the message, then a description someone can start work from. Do not create the task; the user reviews the draft and creates it.
{{- end}}
If you cannot answer, call ` + "`complete_stage`" + ` with ` + "`outcome=errored`" + ` and ` + "`error_reason=<short reason>`" + `.`

var compiledHeader = template.Must(template.New("header").Parse(promptHeader))
var compiledReleaseHeader = template.Must(template.New("releaseHeader").Parse(releaseHeader))
var compiledChatHeader = template.Must(template.New("chatHeader").Parse(chatHeader))
var compiledWiki = template.Must(template.New("wiki").Parse(wikiSection))
var compiledSkills = template.Must(template.New("skills").Parse(skillsSection))
var compiledReminder = template.Must(template.New("reminder").Parse(completeStageReminder))
var compiledImpl = template.Must(template.New("impl").Parse(implementationInstructions))
var compiledBrainstorming = template.Must(template.New("brainstorming").Parse(brainstormingBody))
var compiledDesign = template.Must(template.New("design").Parse(designBody))
var compiledImplPlan = template.Must(template.New("implPlan").Parse(implementationPlanBody))
var compiledReleaseNotes = template.Must(template.New("releaseNotes").Parse(releaseNotesBody))
var compiledChat = template.Must(template.New("chat").Parse(chatBody))

// RenderPrompt renders the per-stage prompt for the agent, ending with the
// complete_stage reminder. Stage bodies substitute task.Vocab into their
// ALLOWED/FORBIDDEN lists; an unset Vocab falls back to Claude Code.
func RenderPrompt(task Task) (string, error) {
	if task.Vocab == (ToolVocab{}) {
		task.Vocab = ToolVocabClaudeCode
	}

	header := compiledHeader
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
	case StageReleaseNotes:
		if task.Release == nil {
			return "", fmt.Errorf("release notes stage without a release")
		}
		header, body = compiledReleaseHeader, compiledReleaseNotes
	case StageChat:
		if task.Chat == nil {
			return "", fmt.Errorf("chat turn without a chat context")
		}
		header, body = compiledChatHeader, compiledChat
	default:
		return "", fmt.Errorf("unknown stage: %q", task.Stage)
	}

	var buf bytes.Buffer
	if err := header.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering header: %w", err)
	}

	if err := compiledWiki.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering wiki section: %w", err)
	}

	if err := compiledSkills.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering skills section: %w", err)
	}

	if err := body.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering %s body: %w", task.Stage, err)
	}

	if err := compiledReminder.Execute(&buf, task); err != nil {
		return "", fmt.Errorf("rendering completion reminder: %w", err)
	}
	return buf.String(), nil
}
