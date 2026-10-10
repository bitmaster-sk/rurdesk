---
title: Wiki
description: A versioned project wiki and a shared wiki for the whole instance, written in Markdown, linked to tasks, and safe to edit at the same time.
---

# Wiki

The **wiki** is where a team writes down what it does not want to explain twice:
conventions, architecture, decisions, runbooks. Pages are Markdown, every save
is kept as a version, and pages link to each other and to tasks.

Open it from **Project → Wiki** in the left menu. While you are in the wiki the
main menu shrinks to icons, so the page tree fits next to it and Board, Table and
the other views stay one click away. Hide the tree with the button next to
**+ Page** when you want the full width for reading; the button that brings it
back sits at the top left of the page, and the browser remembers the choice.

![A wiki page with the page tree on the left, a callout and a table in the text, and the linked tasks on the right](../../site/assets/img/wiki-page.png)

## Spaces

Every page lives in one of two spaces:

| Space | Visible from | Use it for |
| --- | --- | --- |
| **Shared** | every project of the instance | rules that hold everywhere: commit style, security, review process |
| **This project** | this project only | the project's architecture, conventions, runbooks |

Both spaces show up in the same tree, so you never have to switch places to
find a page.

## Pages and the tree

A project owner can make any page of the project the **wiki home page** with
**Set as home** next to **History**. Opening **Wiki** from the project menu
then goes straight to that page; **Unset as home** goes back to the start
screen.

Pages are shown at a comfortable reading width; the full width button next to
**History** stretches the text across the screen, and the browser remembers
the choice. Pages nest without a depth limit. Long titles wrap instead of being cut, and
each level has a guide line, so deep trees stay readable. Drag a page within its
level to reorder it, or use **Move** on the page to put it under another parent.

Each page has a URL built from its title (`System overview` →
`system-overview`; accents are dropped, so `Café` → `cafe`). Renaming a page
keeps its URL, so links never break.

## Writing

The editor shows Markdown on the left and a live preview on the right.
The toolbar above the text adds headings, bold, italic, strikethrough, code,
links, lists, checklists, quotes and code blocks. It wraps the selected text,
or inserts the markup at the cursor; pressing a button again removes it.
**Ctrl/⌘+B**, **Ctrl/⌘+I** and **Ctrl/⌘+K** make text bold, italic or a link.

![The wiki editor: toolbar, Markdown on the left, live preview in the middle and agent access, summary and change note on the right](../../site/assets/img/wiki-editor.png)

The **table** button opens a grid: move over it to pick the number of rows and
columns, then click. The **block** button wraps the current lines in an info,
warning or error block. Blocks use the GitHub syntax, so the same text renders
the same way on GitHub:

```markdown
> [!WARNING]
> Run the migrations before you deploy.
```

`[!NOTE]` is an info block, `[!WARNING]` a warning and `[!CAUTION]` an error.
`[!TIP]` and `[!IMPORTANT]` work too.

| You type | You get |
| --- | --- |
| `[[Page title]]` | a link to a page; the editor suggests titles as you type |
| `[[Page title\|label]]` | the same link with your own text |
| `[[shared:Page title]]` | a link to a page in the **Shared** space |
| `[[Page title#Heading]]` | a link to a heading on another page |
| `[[#Heading]]` | a link to a heading on the same page; the editor suggests headings |
| `[text](#heading)` | a plain Markdown link to a heading on the same page |
| `#123` | a link to task #123; the editor suggests tasks as you type |

Every heading gets an anchor made from its text the same way as page URLs
(`## Rollback plan` → `#rollback-plan`; repeated headings get `-1`, `-2`).
Hover a heading and click **#** to copy a link straight to it.

A link to a page that does not exist yet is shown in red. Clicking it opens the
form for a new page with that title.

The panel on the right holds the page's **parent**, **agent access**, a short
**summary** and a **change note** that ends up in the history.

## Diagrams

A fenced code block tagged `mermaid` renders as a diagram instead of a code
block:

    ```mermaid
    flowchart LR
        idea --> build --> deploy
    ```

The editor preview draws the diagram live while you type; a diagram you did not
change keeps showing without a flash. Supported types include **flowchart**,
**sequence**, **state**, **ER** and **gantt** diagrams (plus class, pie and the
other mermaid diagram kinds — mermaid's own documentation covers their syntax).
Diagrams pick up the app's colours, and a diagram wider than the page scrolls
sideways inside its box instead of stretching the text.

When the source of a diagram is broken, the page never shows a blank area: it
shows an error box with the source text underneath, so you can see what went
wrong and fix it. The page keeps the text exactly as you typed it, so the agent
can still read and edit diagrams as plain text.

## History

Every save creates a new version with its author, time and note. Nothing is
ever overwritten. **History** lists all versions and shows what changed between
each version and the one before it. **Restore vN** does not delete anything: it
saves the old text as a new version.

![The history of a page: versions with their author and note, and the lines that changed between them](../../site/assets/img/wiki-history.png)

## Editing at the same time

Two people can work on the same page without losing anything.

- **You see who else is editing.** Their avatar appears next to the save
  button, with a note under the toolbar.
- **You hear about their save right away.** When someone saves while you are
  writing, a bar tells you who saved which version. **Merge now** pulls their
  changes into your text without touching yours.
- **Saving merges automatically.** If you both changed different parts of the
  page, your save simply produces a new version that contains both.
- **Only real overlaps need a decision.** If you both changed the same lines,
  the conflict screen shows your version next to theirs for just those lines.
  Pick **Use mine**, **Use theirs**, **Both**, or edit the result by hand;
  everything else is already merged.
- **Your draft is kept on the server.** The editor saves a draft every few
  seconds. If the browser crashes or you close the tab, you get the draft back
  the next time you open the editor.

## Trash

**Delete** moves a page, together with all its subpages, to the **Trash** at the
bottom of the tree; the number next to it shows how many items it holds. From
there, **Restore** puts the whole subtree back where it was. If the original
parent no longer exists, the pages return to the top level.
Pages in the trash are deleted for good after **30 days**; a project owner can
delete them sooner.

## Search

The search box at the top of the tree looks through titles, summaries and
text of both spaces. The start of a word is enough: `depl runb` finds
"Deploy runbook". Results show the matching passage.
Wiki pages also appear in the command palette (<kbd>⌘K</kbd> / <kbd>Ctrl K</kbd>).

## Wiki pages on a task

A task's detail has a **Wiki** panel with the pages linked to it:

- every `[[link]]` in the task description is linked automatically and kept in
  sync when the description changes;
- you can add or remove other pages by hand with **+** in the panel. It opens
  the page tree of both spaces; type part of a title to filter it.

![The Wiki panel on a task with a page added by hand](../../site/assets/img/wiki-task-panel.png)

A page in turn lists its **linked tasks** in the panel next to its text: open
tasks first, newest at the top, and closed ones greyed out with their state.
The panel shows 20 at a time; **Load more** brings the next ones. The pages that
link here, under **Linked from**, work the same way.

The task description and comments understand the same `[[links]]` as the wiki.
Type `[[` and the editor suggests pages as you type; press **Enter** to insert
one. The link opens the page, and a link to a page that does not exist yet is
shown in red. Links in comments do not add the page to the **Wiki** panel; only
the description and pages added by hand do.

![A comment being typed with a wiki link started, and the matching pages suggested above it](../../site/assets/img/wiki-comment-link.png)

## Agent access

Each page has one of three access levels for AI agents:

| Level | What an agent gets |
| --- | --- |
| **Always reads** | the whole page in the prompt of every agent run in the project |
| **On demand** (default) | the title and summary in the wiki index of the prompt; the agent opens the page when it needs it |
| **Hidden** | nothing: not in the prompt, not in the index, and not when the agent searches or opens pages |

Pages marked **Always reads** share a token budget per project (12 000 by
default). The editor shows how much of it is used and does not let you go over.
A project owner changes the budget in **Project settings → Wiki**.

### What goes into an agent's prompt

Every stage of an agent run starts with a wiki part built from both spaces, in
this order:

1. the pages linked to the task, whole, even when they are **On demand**;
2. the **Always reads** pages, whole;
3. an index of the other pages: slug, title and summary.

**Hidden** pages are left out, even when they are linked to the task. A good
summary is what makes an **On demand** page useful: it is all the agent sees of
the page until it decides to open it.

All of it together stays within the project's token budget, counted as about
four characters per token. A page that does not fit is not sent; it goes to the
top of the index marked as not loaded, so the agent can still open it. When the
index does not fit either, the prompt says how many more pages there are. However
large the budget, the wiki part of a prompt never goes over 60 000 tokens.

The prompt asks the agent to follow the pages, to trust the code when a page
says something else and point that out in its output, and never to edit the
wiki itself. In the implementation stage it may [propose changes](#agent-proposals)
instead.

### Reading during a run

While it works, the agent can search the wiki and open more pages, including an
older version of a page. It gets the same pages a person sees except
**Hidden** ones. Agents never write the wiki; the most they can do is propose a
change for a person to accept.

### What the agent read

The **Agent run** card on the task shows a **Wiki context** with one line per
stage. The line sums up the stage: how many pages were in the prompt and in the
index, how many the agent opened or searched for, and how many tokens it read.
Open the line to see:

- **Prompt**: the pages that went into the prompt, with their version and whether
  they were sent because they are **always** read or **linked** to the task;
- **Looked up**: what the agent searched for and opened during the stage, with the
  version and size of each page it read;
- **Index**: the pages offered by title and summary only.

Each page opens in the wiki. For a stage that ran more than once, the card shows
the latest attempt.

## Agent proposals

When the code an agent writes makes a page wrong, or adds something the wiki
should describe, the agent proposes a change in the implementation stage: a new
page, an edit, a move under another parent, or a delete. Nothing in the wiki
changes until a person approves the proposal and the code is merged.

A proposal belongs to the agent run. When the agent works on the run again, for
example after a review comment on the pull request, it gets its open and
approved proposals in the prompt and updates them to match the code; it never adds
a second proposal for the same page. Changing an approved proposal takes the
approval back, so a person looks at the new text.

You can decide as soon as the proposal appears, while the pull request is still
open. Approving does not write the wiki yet: the change is published when the pull
request is merged, so the wiki never describes code that is not in the main
branch.

| State | When |
| --- | --- |
| **open** | the pull request is still open and nobody has decided yet |
| **approved · publishes on merge** | a person approved it; it is published when the pull request is merged |
| **ready to publish** | the pull request was merged before anyone decided; the author and the assignee of the task get a notification |
| **needs resolving** | it was approved, but the page changed meanwhile and the edits overlap, so nothing was written; the person who approved it gets a notification |
| **published** | the change is in the wiki |
| **rejected** | a person rejected it |
| **discarded** | the run ended without a merged pull request, even if the proposal was approved |

Each proposal shows up as a **Wiki proposal** card in the task's activity.
Proposals of the whole project are under **Agent proposals** below the wiki tree,
with the number of proposals that need a person after the merge (ready to publish
or needs resolving) next to it, and can be filtered by space.

![Agent proposals: the proposals to review on the left, and a proposal ready to publish with the agent's reason, the change against the version it was based on and the Publish, Edit and publish and Reject buttons](../../site/assets/img/wiki-proposals.png)

The proposal page shows the agent's reason and, for a new or edited page, the
change against the version the agent based it on. If the page changed since, the
page says whether the edits merge cleanly. Someone who may edit the space can:

- **Approve** (before the merge) or **Publish** (after it): the change is saved as
  a new version with the person who decided as its author and the note
  *Agent proposal from #task (run #run)*. When the page changed meanwhile, both
  changes are merged the same way as when [two people edit at
  once](#editing-at-the-same-time). A move places the page at the end of its new
  parent, and a delete moves the page and its subpages to the trash.
- **Edit and approve** / **Edit and publish**: opens the proposal in the editor;
  saving decides it with your text. Overlapping edits found while deciding also
  end up here, to be resolved side by side.
- **Resolve**: for a proposal that needs resolving, opens the editor with the
  overlap; saving publishes it. A move or delete has nothing to edit: **Publish**
  tries again, for example after a missing parent page is restored.
- **Reject**: with an optional reason. The reason is posted as a comment in the
  task, so the agent sees it the next time it works on the task.

## Permissions

| Action | Project space | Shared space |
| --- | --- | --- |
| Read and search | viewer and up | everyone |
| Create, edit, move, delete to trash, restore | member and up | member and up in any project |
| Set **Always reads**, change the token budget, delete for good | owner | admin |
| Approve, publish or reject an agent proposal | member and up | member and up in any project |
