---
title: Agent skills
description: Named Markdown instructions injected into an agent run's prompt, per stage.
---

# Agent skills

A **skill** is a named piece of Markdown that gets injected into an agent's
prompt for a given stage of a run. Skills are how you tell every agent, once,
the things that are true of *your* project: how tests are run, what "done"
means, which conventions to follow.

Without them each team's agents rediscover — or ignore — those rules on every
run. That is usually what is behind an agent that pushes code the pipeline
rejects.

Skills are text, not enforcement. They make an agent far more likely to run your
checks; they do not stop it from finishing a stage. Treat them as the standing
instructions you would give a new colleague.

## The catalog

The catalog is global to the instance and managed by an admin under
**Administration → Agent skills**. Each skill has a name, a short description
and its Markdown content.

Some skills ship with the application (**builtin**). You can edit a builtin —
it is then marked **edited** — and **Restore original** puts the shipped text
back at any time. Builtins cannot be deleted; skills you create yourself can.

An application update adds skills you don't have yet, and improves the wording of
the builtins you have left untouched. **A builtin you edited is never
overwritten** — it stays exactly as you wrote it and keeps its *edited* mark
until you restore the original, which also opts it back into future updates.

### What ships out of the box

| Skill | Enabled by default | What it does |
| --- | --- | --- |
| **Repository rules** | design, plan, implementation | Tells the agent to read `AGENTS.md` (or `CLAUDE.md`, `.cursor/rules`) from the repository root and treat it as binding — and, when there is none, to derive the project's conventions from CI config, `Makefile` or `package.json` rather than guessing. |
| **Verification rules** | implementation | Find the project's checks, run them in full, read the real output, and never push a branch whose tests, linter, formatter or build are failing. If a check cannot be made to pass, the stage ends with an error instead of a red branch. |
| **Testing rules** | off | Write the failing test first, then the code that makes it pass. Opinionated — switch it on per project if that is how your team works. |
| **PR rules** | off | Commit-message and pull-request-description conventions. |

The two enabled-by-default skills are switched on for a project when the project
is created. Turning one off is permanent for that project — an update will not
switch it back on.

## Per-project defaults

Every project decides which skills apply to which stage, in **Project settings →
Agent skills**. The panel is a matrix: skills down the side, the four stages
(brainstorming, design, implementation plan, implementation) across the top.
Ticking a cell saves immediately.

Only project owners can change the matrix; any project member can read it,
because the assignee dock (below) shows it.

New runs pick up the matrix at the moment the run is created.

## Per-run overrides

The project matrix is the default, not a straitjacket.

**When you assign an agent**, open the skills dock from the agent's row in the
assignee dropdown (the sliders icon). It shows the per-stage chips pre-filled
from the project matrix; adjust them and press **Assign**. The run is created
with exactly those skills. Clicking the agent's name instead assigns it straight
away with the project defaults — one click, no detour.

**While a run is going**, the **Skills** chip on the run card opens the same
per-stage list. Stages that have already started are locked — their prompt was
already built and sent — and the rest can still be changed.

## Agent workload

The same dropdown shows what each agent is doing: a green or orange dot with
"idle" or "working · N queued". The info icon opens a read-only panel with the
run it is on right now, its queue, runs completed today, tokens used over the
last 7 days, average run duration and failed attempts.

Counters cover the agent's whole workload across projects; the named tasks are
only ever from the project you are looking at.

## What agents receive

At each stage the tracker resolves that stage's skills and sends them to the
gateway with the rest of the stage context. The gateway renders them into the
prompt under a `## Skills` heading, ahead of the stage instructions, stating
that they are mandatory for the stage.

A skill's content therefore goes to whichever AI provider your gateway is
configured with. Do not put secrets in skills.
