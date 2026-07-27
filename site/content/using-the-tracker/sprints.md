---
title: Sprints
description: Time-boxed cycles on the board — plan work into a sprint, track points, and roll unfinished work into the next cycle.
---

# Sprints

A **sprint** (or cycle) is a time-boxed iteration you plan work into. Sprints
are **owned by the project** and live right on the **Board** — there is no
separate screen to learn. Each task can belong to at most one sprint of its
project, and carries **story points** that drive the sprint's totals.

## The sprint scope chip

The Board toolbar has a **sprint scope chip** next to the layout switch. Use it
to focus the board on one cycle:

- Pick a sprint → the board shows only that cycle's tasks.
- Pick **All cycles** → the scope is cleared.
- **＋ New sprint** → creates a cycle for the current project. The name
  (`Sprint 1`, `Sprint 2`, …) and a two-week date window are filled in for you,
  so a new cycle is **one click**. You can rename or re-date it later.

There is no manual "activate" step — the **current** cycle is derived from the
dates (the cycle whose window contains today, otherwise the next planned one).

## The Sprints board layout

The layout switch offers **Columns · Swimlane · Sprints**. Switching to
**Sprints** stacks your cycles as rows and your project's states as columns:

- The **current** cycle is the top lane, then upcoming planned cycles, then a
  **Backlog** lane for work not yet assigned to any cycle.
- Drag a card **up or down** to move it between cycles (or into the Backlog).
- Drag a card **left or right** to change its state, exactly like the normal
  board.

In Sprints mode the scope chip shows **All cycles** and is disabled — the lanes
already show every cycle, so the chip's filter would only get in the way. Your
previous scope selection comes back when you switch to Columns or Swimlane.

## Points

Set **points** on a task from its detail panel (next to the time estimate).
Points are dimensionless story points, separate from the time estimate, and
feed the sprint's capacity and velocity. Agents can set points too, via the
`create_issue` / `update_issue` MCP tools (see [Bots & agent runs](../agentic-workflow/agents.md)).

## Closing a sprint & rollover

When a cycle ends, **close** it. Closing:

- moves every **unfinished** task (any state that isn't a *final* state) into
  the **next planned** cycle, or clears it to the **Backlog** if there is none;
- leaves **finished** tasks in the closed cycle, so its velocity stays honest;
- is final — a closed cycle can't be closed again.

Velocity for a cycle is the sum of points of its tasks that reached a final
state.

## Viewing closed sprints

Closed cycles are hidden by default. To see them, open the Board's **settings
popover** (the adjust button in the toolbar) and switch on **Show closed
sprints**. Closed cycles then appear at the end of the sprint strip, muted,
most recently ended first.

A closed cycle is immutable history:

- clicking its tab shows its tasks, and you can still drag a task **out**
  (to an open cycle or the Backlog) — for example to rescue a ticket that was
  left behind;
- nothing can be dragged **into** a closed cycle, and it can't be edited or
  rolled over.

The setting is a personal display preference — it is remembered per browser
and doesn't affect anyone else.
