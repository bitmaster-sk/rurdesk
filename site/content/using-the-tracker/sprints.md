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

A cycle must end after it starts; the dialog rejects anything else.

A closed cycle is immutable history:

- clicking its tab shows its tasks, and you can still drag a task **out**
  (to an open cycle or the Backlog) — for example to rescue a ticket that was
  left behind;
- nothing can be dragged **into** a closed cycle, and it can't be edited or
  rolled over.

The setting is a personal display preference — it is remembered per browser
and doesn't affect anyone else.

## Sprint health

Under the sprint strip sits a one-line **health strip**. It always shows
something — on the Backlog tab it counts the work that is still open and in no
cycle, so the board never jumps as you switch tabs.

For a cycle it shows the name and window, a segmented bar (done / in progress /
not started), the done-out-of-total figure, where you are in the window, the
pace so far against the pace needed, and a verdict chip. A **Points / Tasks**
toggle on the right switches every number between story points and task counts.

The chips mean:

- **on track** — at the current pace the commitment is projected to be met
  (within 5 %);
- **behind N** — the projection falls short by roughly N points or tasks;
- **too early to forecast** — fewer than three days have elapsed, so no pace and
  no forecast are shown;
- **over-committed vs avg N** — the cycle hasn't started and its commitment is
  more than 15 % above the average of the recent closed cycles. How many cycles
  that average covers is an instance setting (**Cycles averaged for velocity**,
  ten by default). Cycles that finished nothing in the unit being shown are left
  out of that average, so cycles you ran before adopting points do not drag the
  points baseline down.

### What it does not claim

- The forecast is a straight-line projection from the pace so far. It is not a
  model of your team, and it says nothing at all for the first two days.
- With no points set anywhere in the cycle, the strip counts **tasks** instead
  and says so. With only *some* tasks pointed, the points figure covers only
  those tasks — it is not scaled up to the rest.
- A closed cycle shows its **window**, not the moment it was closed; the closing
  time isn't recorded.
- Until the burndown and velocity charts ship, a closed cycle shows only what
  was **finished**, with no out-of-total fraction, because unfinished work has
  already rolled into the next cycle and is no longer counted here.
- The strip updates live for changes **you** make, and for task changes your
  teammates make in the same project — a burst of them is collapsed into a single
  refresh. Creating, renaming or deleting a *cycle* elsewhere reaches you on your
  next refresh.
- While the numbers for a cycle are still loading, the strip shows only its name
  and window rather than a stale count from the tab you came from.
- With no points anywhere in the cycle the **Points / Tasks** toggle is locked to
  Tasks — there is nothing for the points view to show.
- The **Backlog** row counts only work that is still open, so a finished task that
  never belonged to a cycle is left out of both figures — the board below it still
  lists that task, since the tab shows every state. The row prints tasks and points
  side by side, so the unit toggle is hidden there.
- A closed cycle draws a **full** bar. It reports what was finished, so reopening
  a task afterwards changes the figure rather than leaving an unexplained gap.
- When **Save** is greyed out in the cycle dialog, hovering it says which field is
  the problem.
