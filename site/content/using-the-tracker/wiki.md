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

A page in turn lists its **linked tasks** in the panel next to its text.

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
| **On demand** (default) | the title and summary in the wiki index; it reads the page when it needs it |
| **Hidden** | nothing; the page is meant for people only |

Pages marked **Always reads** share a token budget per project (12 000 by
default). The editor shows how much of it is used and does not let you go over.
A project owner changes the budget in **Project settings → Wiki**.

> Agents start reading the wiki in an upcoming release. Until then, these
> settings are saved and shown, but agent runs do not use the wiki yet.

## Permissions

| Action | Project space | Shared space |
| --- | --- | --- |
| Read and search | viewer and up | everyone |
| Create, edit, move, delete to trash, restore | member and up | member and up in any project |
| Set **Always reads**, change the token budget, delete for good | owner | admin |
