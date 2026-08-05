---
title: Saved views
description: Save a filter, sort and view type under a name, keep it private or share it with the project, and reopen it in one click or from a link.
---

# Saved views

A **saved view** is a named combination of *what you are filtering by*, *how it is
sorted*, and *which view you are looking at* — table, Kanban, Calendar or Gantt. Set
the filters once, save them as "My open bugs" or "Needs review", and reopen that exact
list in one click.

Saved views belong to the **project** and are reachable from the **Views** button in
the task page toolbar, on every view.

## Saving a view

1. Filter and sort the list the way you want it.
2. Open **Views** → **Save current view…**
3. Give it a name and choose whether it is private or shared.

The view records the state of the toolbar at that moment. Later changes to your filters
do **not** update it — use **Update "&lt;name&gt;"** in the same menu when you want to
overwrite it.

## Applying a view

Click a view in the list. If it was saved on a different view type, you are taken there
— that is why every row carries a **Table / Board / Calendar / Gantt** badge.

Applying a view **replaces** your current filter rather than adding to it: anything the
view does not filter on comes back unfiltered. The applied view is highlighted, and its
name shows on the toolbar button. Press the **✕** next to the name to stop following it —
the list goes back to what you would see landing on that view normally.

Editing the filter or the sort while a view is applied does **not** drop it: the button
gets a dot to say the view has unsaved changes, and **Update "&lt;name&gt;"** writes them
back. Clicking the view again discards them.

Typing in the search box narrows the list by name. When exactly one view is left,
**Enter** applies it.

## What a view stores — and what it deliberately does not

Stored:

- the filters from the filter bar — title, states, severities, assignees, and the
  **Created** / **Last update** date filters;
- the sort column and direction;
- the view type, plus the Kanban layout (columns or swimlanes).

Not stored, on purpose:

- **the Kanban sprint tab** — it would go stale the moment that cycle closes;
- **ad-hoc task selections**;
- **the Calendar month and the Gantt window** — those are always computed around today,
  so a view saved in July does not pin you to July forever.

Date filters follow the same rule. A **rolling window** (*Last 7 / 30 / 90 days*) stays
rolling: a view saved today still means "the last 30 days" next month. A **Custom
range** of two fixed dates stays fixed — that is a deliberate choice, like "everything
since the 1.0 release", not an accident.

Two filter flags — *unscheduled tasks* and *unassigned tasks* — can be present in a
view (for example from a link an agent produced) and **are** applied, but the filter bar
has no control for them, so it will not show them. They survive further edits to the
filter.

## Private and shared views

A **private** view is visible only to you. A **shared** view is visible to everyone in
the project. Switch between the two with the lock / people button on the row, or in the
save dialog.

- Anyone who can create tasks can create views.
- A view can be renamed, re-shared or deleted by **its creator** or by a **project
  owner**. Everyone else sees only the copy-link action.
- Applying someone else's shared view never changes it.

## Links and the command palette

**Copy link** on a row gives you a URL ending in `?view=<id>` — open it and the view is
applied on arrival. Applying a view also puts that parameter in your address bar, so
copying the URL out of the browser works just as well. A link to a view that no longer
exists (or belongs to another project) reports that and drops the parameter.

Saved views also appear in the command palette (`⌘K` / `Ctrl+K`) under a **Views**
heading in the `/` navigation list — there is no separate prefix to learn. They are
there even if you have not opened the task list yet in this session.

## When the data behind a view changes

A view stores ids, not names. If a state, severity or member it filters on is deleted
later, the view still opens — it simply matches fewer tasks (possibly none). Nothing is
validated or rewritten on your behalf, so a view never silently starts meaning
something else.
