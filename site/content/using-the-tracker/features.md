---
title: Features
description: Day-to-day operation — tasks, views, relations, tracker, and AI features.
---

# Features

How to use the platform day to day. For the agent workflow see
[Agents](./agents.md).

## Project overview

Opening a project lands on its **overview dashboard** — a read-at-a-glance
summary of where the project stands:

- **Tasks by state** and **Tasks by severity** — how the backlog is
  distributed
- **Estimated vs Tracked** — planned effort against logged time
- **Workload by Assignee** — open (non-final) tasks per person, each row with
  the assignee's avatar, name, a proportional bar, and the open-task count
  (unassigned tasks are grouped last)
- **Pinned tasks** — quick links to the tasks you pinned to the project

![Project overview dashboard](../../site/assets/img/project-overview.png)

## Tasks

Tasks are the core unit of work. Create, edit, assign, and move them through
states. Each task has a state, severity, type, tags, and a tracker, and can carry
relations to other tasks.

![Task detail panel](../../site/assets/img/issue-detail.png)

### Resizing the detail panes

The task detail is split into task info on the left and activity on the right.
Drag the separator between them to give one side more room — the form reflows to
fewer columns as its pane narrows, and neither side can be dragged below a
readable minimum.

Hovering the separator reveals three controls. The two arrows collapse a pane
entirely, so you can read a long description or a long discussion full width;
the opposite arrow brings it back. The button between them resets to the even
50/50 split and dims once you are already there — so there is always a visible
way back, whatever you dragged or collapsed. Hovering any of the three explains
what it does, including why it is currently inactive.

Double-clicking the separator resets it too. With the separator focused, the
arrow keys move it (hold Shift for a finer step), Home/End collapse a side, and
Enter resets.

Your split is remembered in the browser and applies to every task you open.

### Participants & notifications

Every task keeps a list of **participants**. You become one automatically when
you create the task, are assigned to it (bots included), comment on it, or are
@mentioned. You can also add any project member manually. Participants are never
removed.

Comment notifications go **only to participants** (not to every project member).
Each participant can silence a single task for themselves with the 🔕 toggle —
this only affects your own notifications, never anyone else's. The participant
list updates live over WebSocket.

![Participants card in the task detail](../../site/assets/img/participants-panel.png)

### Mentions

Type `@` in a task comment or any chat message to open a mention picker — it
lists project members (in task comments), team members (in team chat), or the
peer (in direct messages). Click to insert a mention as a structured token
`@[Name](user:<id>)`.

![Mention picker autocomplete in the comment composer](../../site/assets/img/mention-picker.png)

- **In task comments:** mentioning a user **adds them as a participant** (if not
  already) and sends them a **Mention notification**
- **In team/project chat and DMs:** mentions render as **visual chips** (styled
  links) for reference, with no additional notification

![A posted comment with a rendered mention chip](../../site/assets/img/mention-chip.png)

Mentions are reliably detected by user ID — no false positives from similar names,
case sensitivity, or word boundaries.

### Views

The same tasks can be viewed five ways — switch views from the project toolbar:

#### Filters

The filter bar is the same in every view — table, Kanban, Calendar, and Gantt —
so a filter you set carries over when you switch views.

**Created** and **Last update** filter by date: pick a rolling window (*Last 7 /
30 / 90 days*) or a **Custom range** of two fixed dates. A rolling window keeps
rolling — set to *Last 30 days*, it still means the last 30 days next month.

#### Quick actions (right-click)

Right-clicking a task opens the same **quick-actions menu** in every view — table,
Kanban, Calendar, and Gantt — so you can edit without leaving the view or opening
the task:

- **State**, **severity**, **type**, and **assignee** — inline selectors
- **Reschedule** — previous/next day, today, remove the date, or pick one from a
  calendar
- **Split** the task with AI (see [Task Split](#task-split))
- **Open** the task, **copy its ID**, or **delete** it

On the Kanban board the reschedule section is hidden — drag the card instead.

![Quick-actions menu opened on a task in the table](../../site/assets/img/quick-actions.png)

#### Task table

A fast, filterable grid. Sort and filter across task fields.

![Task table with filters](../../site/assets/img/view-table.png)

#### Kanban

A drag-and-drop board grouped by state. Move a card between columns to change its
state.

![Kanban board](../../site/assets/img/view-kanban.png)

#### Calendar

Tasks placed on a calendar by their schedule (FullCalendar). Drag to
reschedule.

- **Keyboard**: <kbd>t</kbd> or <kbd>Home</kbd> jumps to today,
  <kbd>←</kbd>/<kbd>→</kbd> step to the previous/next period, and
  <kbd>+</kbd>/<kbd>-</kbd> change granularity (month → week → day)

![Calendar view](../../site/assets/img/view-calendar.png)

#### Gantt

A full project timeline:

- Task bars on a zoomable timeline
- **Dependency arrows** between related tasks, drawn by dragging the
  **connection handle** on one bar's edge onto another bar
- **Drag** a bar to move it or **resize** its edge to change duration —
  dependent dates **cascade** to stay valid
- **Schedule** a backlog item by dragging it from the WBS side panel onto the
  timeline
- **Reorder rows** by dragging a scheduled task up or down in the WBS side
  panel — the manual order is saved and persists across reloads. Dependency
  arrows are independent of row order.
- **Critical path** highlighting
- **Minimap** and a **WBS** (work-breakdown) side panel
- **Keyboard**: <kbd>t</kbd> or <kbd>Home</kbd> jumps to today (also a **Today**
  toolbar button), <kbd>+</kbd>/<kbd>-</kbd> zoom, <kbd>←</kbd>/<kbd>→</kbd> pan

![Gantt timeline with dependency arrows](../../site/assets/img/view-gantt.png)

### States, severities & task types

Project settings let you add, rename, reorder, and delete the states,
severities, and task types tasks move through. Deleting one that's still in use
doesn't silently orphan data:

- If the state, severity, or type is **unused** — no tasks reference it, it
  isn't the project default, and (for states) no workflow event maps to it —
  it's removed immediately after a plain confirmation.
- If it's **in use**, a dialog shows how many tasks are affected (plus
  whether it's the project default or mapped to a workflow event) and asks you
  to either **migrate** everything to another state/severity/type or explicitly
  **unassign** it. Nothing is deleted until you choose.
- If it's the **last** one of its kind in the project, there's nothing to
  migrate to, so the dialog only offers unassign.

#### Task types

A task's **type** says what kind of work it is — new projects start with
**Bug**, **Feature**, and **Task**, and you can add your own (Spike, Chore,
Support, whatever fits). The type is **optional**: a new project has no default
type, so tasks stay untyped until you pick one. Set a default in project
settings if you'd rather every new task start with one.

Types are deliberately monochrome. Colour on a task already means two things —
the state badge and the severity dot — so a third colour would make all three
harder to read at a glance. The type renders as a plain outlined label instead.

You'll see the type in the **task table** (its own sortable column, ordered the
way you arranged the types in settings), on **Kanban cards**, in the **task
detail**, and in the **quick-actions** popover. It is deliberately left out of
the Calendar and Gantt views, where an event or a short bar has no room for it
without pushing the task title out.

Filter by type in the filter panel — including a **"tasks with unset types"**
toggle — and save that filter into a view like any other.

### Automatic state changes

**Project settings → Automatic state changes** (project owner only) maps
**workflow events** to task states: when an event fires, the task's state
updates to whatever you mapped it to; leave an event unmapped to skip it. The
events are grouped by what can trigger them:

- **Any linked pull request** — only the `done` event ("the pull request is
  merged"), because it fires for **both** an agent-opened PR and a **manually
  linked** one. A background check runs roughly every **60 seconds**: once a
  linked PR/MR is merged, the mapped task picks up the `done` mapping's state.
  For **manually linked PRs**, a task already in a **final** state is left
  alone — the PR status pill still shows **Merged**, but the task's state does
  not change. **Agent runs apply the mapping regardless of the task's current
  state.** A PR/MR that is **closed without merging** changes nothing.
- **Agent runs only** — the other seven events (`queued`, `in_progress`,
  `awaiting_input`, `awaiting_approval`, `pr_open`, `failed`, `cancelled`) fire
  **only** for [agent runs](../agentic-workflow/agents.md). Linking an
  already-open PR to a task does **not** fire `pr_open` — that stays an
  agent-only signal, so manually linking a PR never jumps the state to
  whatever you mapped `pr_open` to.

## Relations & scheduling

Link tasks to express how work connects:

| Relation | Meaning |
| --- | --- |
| **Hierarchy** | Parent / child breakdown |
| **Schedule** | One task must come before another (drives Gantt arrows & cascade) |
| **Duplicate** | Marks duplicates |
| **Relates-to** | Loose association |

Create schedule relations in the Gantt view by dragging the **connection
handle** on a task bar onto another bar (the side you connect — start vs.
finish — picks the sub-type: finish-to-start, start-to-start, etc.). While
dragging, the target handle lights up and the line snaps onto it — that's the
"release here to connect" cue; dropping anywhere on the bar connects to its
nearer side. Cycles are rejected. Relation validity is also enforced at the database level. Relations
can likewise be added and removed from the task itself.

![Dragging to create a relation](../../site/assets/img/relation-drag.webm)

## Tracker — time tracking

Log time against a task and each entry lands in its **activity feed** — filter
the feed to the **🕐 Time** chip to see just the tracked time, per contributor.

![Tracked time in the task activity feed, filtered to the Time chip](../../site/assets/img/tracker.png)

## AI features

> These run **inside the API** using the provider configured in
> [Configuration](./configuration.md) (`AI_PROVIDER` & friends). They need an AI
> key — or a local **Ollama** for no cloud keys. They are independent of the
> [agent gateway](./gateway.md).

### Agent skills

Named Markdown instructions injected into an agent run's prompt, per stage —
how your project runs its tests, what conventions bind, what "done" means. An
admin manages the catalog, each project picks which skills apply to which stage,
and either can be overridden for a single run. See
[Agent skills](./agent-skills.md).

### Quality Check

When creating or editing a task, the AI scores its quality inline and suggests
improvements (missing context, weak acceptance criteria, etc.). Apply a
suggestion or ignore it — **you approve**. The quality badge is visible to every
user.

![Quality check — low score with suggestions](../../site/assets/img/quality-low.png)

![Quality check — high score after improvements](../../site/assets/img/quality-high.png)

### Project Kickstarter

Describe a project in a few sentences and the AI generates a **staged backlog**:
titles, descriptions, estimates, and schedule relations. Review it in a
**staging view**, then **accept** to save everything in one click. Input length
is capped by `PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH`.

![Project Kickstarter — description to backlog](../../site/assets/img/kickstart.webm)

### Task Split

Click **split** on any task to break it into child tasks. The split happens in
two steps:

1. **Guide the split (optional).** A dialog opens where you can add suggestions
   for *how* the task should be broken down — e.g. "split per UI screen" or
   "one task per endpoint". Leaving it empty lets the AI decide on its own. Then
   click **Generate**.

   ![Split dialog — optional suggestions before generating](../../site/assets/img/split-dialog-before-generate.png)

2. **Review and accept.** The AI returns the proposed child tasks. Review them
   and **accept** to create them — nothing is saved until you confirm.

   ![Split dialog — generated child tasks awaiting acceptance](../../site/assets/img/split-dialog-after-generate.png)

### MCP server

The API exposes its operations as **MCP tools** (`create_issue`, `list_issues`,
`update_issue`, `add_relation`, …), so MCP-capable clients (Claude Code, Cursor)
can drive the backlog natively.

`list_issues` accepts `updated_within` and `created_within` as rolling windows —
`"2h"`, `"30d"`, `"1d8h6m"` (units: `d` = 24h, `h`, `m`, `s`). Prefer them over
computing dates: the server resolves the window, so an agent never has to know
today's date. A value that cannot be parsed is rejected with **422**.

- Full endpoint: `http://<host>/mcp/sse`
- Restricted planning subset: `http://<host>/mcp/plan/sse`
- Authenticate with a bot/user API key (Bearer)

## Command palette & keyboard shortcuts

Press <kbd>⌘</kbd><kbd>K</kbd> (or <kbd>Ctrl</kbd><kbd>K</kbd>) anywhere to open the
**command palette** — one keyboard-driven entry point for navigation, task actions,
and search. Start typing to fuzzy-match; use <kbd>↑</kbd>/<kbd>↓</kbd> to move,
<kbd>↵</kbd> to run, <kbd>⌘</kbd><kbd>↵</kbd> to run without closing, <kbd>⇥</kbd> to
complete the highlighted item into the input, and <kbd>esc</kbd> to clear or close.

Type a **prefix** to scope the search, or jump straight in with a dedicated shortcut:

- <kbd>&gt;</kbd> — **commands** on the open task (set state, set severity, assign, clone)
- <kbd>@</kbd> — **people** in the current project (on a task, picking someone assigns them to it)
- <kbd>#</kbd> — **tasks** (type the number to jump straight to `#428`)
- <kbd>/</kbd> — **navigation** (Overview, Tasks, Board, Calendar, Gantt, Settings,
  switch project) — press bare <kbd>/</kbd> to open the palette already in navigation
- <kbd>?</kbd> — open the **keyboard-shortcuts** help sheet

Searching in the *all* scope with no results offers a **Create task “…”** action that
files a new task titled with what you typed and drops you on its detail to flesh it out.
Actions that change data respect your project role — a viewer sees navigation and search
but not create/edit commands. In the task list, <kbd>j</kbd>/<kbd>k</kbd> move the
highlight and <kbd>↵</kbd> opens the highlighted task. The Gantt and the Calendar share
the same bindings: <kbd>t</kbd> (or <kbd>Home</kbd>) jumps to today,
<kbd>+</kbd>/<kbd>-</kbd> change zoom/granularity and <kbd>←</kbd>/<kbd>→</kbd> pan the
timeline or step between periods. Shortcuts never fire while you are typing in a text
field.

![Command palette open over the task list](../../site/assets/img/command-palette.png)

## Real-time collaboration

- **Messaging** — per-context chat
- **WebSocket notifications** — creating, editing (state, severity, assignment,
  schedule — including Gantt drag/resize cascades), and deleting a task propagates
  live to every connected client: every open view (table, Kanban, Calendar,
  Gantt) reloads and marks the changed task with a short **pulse**; a Kanban
  card that changed column glides there in real time. Comment
  notifications go only to task participants (see
  [Participants & notifications](#participants--notifications)); participant
  list changes also broadcast over WebSocket so the UI updates without a reload

### HTML mockup previews (agent design output)

When a task asks for a UI to be sketched first — either in the **task
description** or in a **comment** ("create a mockup first", "I want to see some
mockups") — the agent attaches a working HTML mockup in its **Design** output
using a fenced ```` ```mockup ```` block. Instead of a wall of code, the design
comment in the **task activity feed** shows a compact **UI Mockup** card; click
it to open the mockup full-size, rendered live.

The mockup runs in a **sandboxed `<iframe>`** with no access to the app, your
session, or the API — its scripts and styles execute in an isolated origin behind
a strict content-security policy, so previewing an AI-proposed design is safe.

    ```mockup title="Login screen"
    <button style="padding:.5rem 1rem">Sign in</button>
    ```

The mockup card renders alongside the agent's design write-up, next to the same
` ```diff ` blocks the plan stage produces. The agent only attaches a mockup when
the task explicitly asks for one.

![UI mockup cards in a design comment — one chosen](../../site/assets/img/message-mockup.png)
