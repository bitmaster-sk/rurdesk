---
title: Agents
description: Add an agent, wire a gateway, and run the AI agent workflow.
---

# Agents

This page covers the human side of the **AI Agent workflow**: creating an agent,
connecting it to a [gateway](./gateway.md), starting a run on a task, and
approving stages.

> Prerequisite: a running [gateway](./gateway.md) container (Goose, pointed at
> your chosen provider) that you can reach from the API over the compose network.

## Concepts

| Term                        | Meaning                                                                                                                         |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| **Agent user**              | A non-human user account the agent acts as. Its writes are attributed to the agent (provenance).                                |
| **Gateway → Tracker token** | The agent's API token; the gateway sends it as `GATEWAY_TO_TRACKER_TOKEN` on every tracker call.                                |
| **Agent gateway**           | The agent's single gateway record (the Goose gateway's URL). Holds the Tracker → Gateway token the tracker signs webhooks with. |
| **Agent run**               | One execution against one task: moves through stages, ends in a PR (or failure).                                                |
| **Stage**                   | A phase of work: brainstorm → design → plan → implement. Some are skippable.                                                    |

## 1. Create an agent user

In **Administration → Users** click **Create user**, toggle **Agent**, give it a
name, and enter its **Gateway URL** — the Goose gateway's base URL, e.g.
`http://gateway-agent-007:9090`. Each agent has exactly **one** gateway
and can work on any project it is a member of.
More detail in [User management](./user-management.md#agents).

![Creating an agent](../../site/assets/img/agent-create.png)

## 2. Copy the agent's credentials

On creation the agent's **credentials dialog** opens with **both one-time tokens**.
Copy each immediately and set it in the gateway container's environment:

| Token                       | Gateway env var                                                         |
| --------------------------- | ----------------------------------------------------------------------- |
| **Gateway → Tracker token** | `GATEWAY_TO_TRACKER_TOKEN` — the gateway sends it on every tracker call |
| **Tracker → Gateway token** | `TRACKER_TO_GATEWAY_TOKEN` — the tracker signs each webhook with it     |

Each token is shown **only once**; the dialog also records the agent's Gateway
URL. If you lose a token, reopen the dialog (key icon on the agent row) and use
**Regenerate token**, then reconfigure the gateway container.

The token row scrolls horizontally, so a long token stays fully readable and can
be selected by hand. **Copy** uses the browser clipboard API, which browsers
expose only over HTTPS (or `localhost`); on a plain-HTTP deployment it falls back
to a legacy copy path, and if the browser blocks that too the dialog says so —
select the token in the row and copy it manually.

![Agent credentials dialog — the one-time Gateway → Tracker and Tracker → Gateway tokens](../../site/assets/img/agent-keys.png)

> The tracker signs each `stage_execute` webhook with the Tracker → Gateway
> token; the gateway verifies the `X-Tracker-Signature` and rejects mismatches
> with `401`. The token must therefore be identical on both sides.

## 3. Start a run on a task

Assign a task to the agent (or trigger the agent action on the task). The API
enqueues a run and fires a `stage_execute` webhook at the gateway. The gateway
checks out a per-run worktree and starts the agent. From there the run drives
itself through the stages — assign, approve the gated stages, and it ends in a
PR:

![The agent workflow end to end](../../site/assets/img/agent-workflow.webm)

### Short workflow for small tasks

When you assign an agent from the task, the dialog has a **Short workflow**
switch. With it on, the run skips the design and implementation plan stages:
the agent still brainstorms (and asks if something is unclear), then goes
straight to implementation and opens the pull request. There is nothing to
approve along the way; the pull request is your review point. Use it for small,
clearly described tasks. In the dialog the skipped stages are greyed out and
marked **skipped**.

You can also flip the switch on the run card until the design (or, in a short
run, the implementation) starts. After that it is locked: the card shows the
workflow as **Full** or **Short** with a lock icon. A restarted run keeps
the workflow it had. While the dialog is open, clicking the agent's name in the
list does the same as **Assign**: the run starts with the skills and workflow
you set. Picking someone else closes the dialog and drops those settings.
Selecting the agent in the assignee field without opening the dialog starts the
full workflow with the project's default skills.

### Choosing the model

One agent can run tasks on different models, so you do not need a second agent
for a cheaper or a stronger model. The dialog lists the models the agent's
gateway offers (its `MODELS`, see [Models](./gateway.md#models)) with the
gateway's default picked. Pick another one for the whole run, for example a
cheap local model for a trivial task and a strong one for an architecture change.

The dialog lists the stages in the order they run. A closed stage shows a summary
of its skills, model and turn limit; open it to change them. Each stage can
override the run's model, so the design can run on a stronger model than the
brainstorming. A stage without its own choice runs the run's model, and a run
without a choice runs the gateway's default.

While the run is going, each stage that has not started yet shows the same
form as the dialog in its card on the run card. Changes save as you make them,
and empty fields use the run's defaults. Once a stage starts, its card shows
what it ran with and can no longer be changed. The run's own model is set only
when you assign the agent.

The dialog offers only the models the gateway reported when it last started. A
gateway that has never reported (an older version, or one that has not started
since the upgrade) shows no model choice and runs its own default. If the
gateway stops offering a model between assigning and running a stage, the stage
runs the gateway's default and its stage card shows a warning naming both
models.

Each stage card shows the model the stage ran with. When a run used more than
one model, the run block splits its tokens by model.

### Turn limits

Each stage runs with a turn limit, the most tool calls and replies the agent
may spend before the stage ends. A stage that runs out ends before the agent
reports completion; press **Continue** to retry it with a fresh budget. The
limit comes from the first of these that is set:

1. the stage's own limit, set on the stage in the dialog or later on the run card;
2. the agent's limit for planning stages or for implementation, set when you
   create the agent or later in its edit dialog in **Admin → Users**;
3. the app settings in **Admin → Settings**, 250 for planning stages and 500 for
   implementation by default.

A weaker model often needs a higher limit to finish. The gateway can cap the
limit with `MAX_TURNS_LIMIT`. It reports the cap to the tracker together with
its models, and a stage never runs with more turns than the cap, whichever
setting asked for them.

### Taking the task away from the agent

Assigning the task to someone else, or clearing the assignee, does not cancel
the run. The stage in progress stops and runs again later; finished stages are
kept. The run card shows **Paused**; hover over it to see who has the task now.
The run no longer counts in the agent's queue.

To continue, press **Assign back** on the card, pick the agent in the assignee
field, or assign it through its dialog. The dialog opens with the paused run's
own skills, workflow, models and turn limits, and applies them to the stages
that have not started yet. If the workflow switch is already
locked, the dialog says so and nothing changes. Assigning a different agent
hands the run over to it.

### Custom fields in a run

If the project defines [custom fields](../using-the-tracker/custom-fields.html),
the agent sees their values on every task it reads, so whatever context a field
carries reaches the run.

## 4. The run lifecycle

A run advances through **phases**:

| Phase               | Meaning                                          |
| ------------------- | ------------------------------------------------ |
| `queued`            | Waiting for the agent to finish its current task |
| `in_progress`       | Agent is actively working a stage                |
| `awaiting_input`    | Agent needs clarification from a human           |
| `awaiting_approval` | A stage finished and waits for human sign-off    |
| `pr_open`           | A pull/merge request has been opened             |
| `done`              | Run completed                                    |
| `failed`            | Run failed (see `errorMessage`)                  |
| `cancelled`         | Run was cancelled                                |

Each **stage** carries its own status: `pending`, `active`, `done`,
`awaiting_approval`, `failed`, or `skipped`. The run card starts with a block
for the whole run: its model, workflow and start time, and its total tokens,
duration and tool calls. Below it, every stage has its own card. The header of
the card shows the stage's status, when it finished and how long it took. Open
a stage that ran to see its attempts, tool calls and tokens, the model, skills
and turn limit it ran with, and which agent executed it after a hand-off. The
stage waiting for your review and the next stage to start are open from the
start. A skipped stage is struck through and has nothing to open.

![Agent run card — run block, stage cards and phase badge](../../site/assets/img/run-card.png)

Each stage card also shows which [wiki](../using-the-tracker/wiki.md#what-the-agent-read)
pages the stage got in its prompt and which ones the agent opened while it
worked. Changes the agent proposes to the wiki appear as separate cards in the
task's activity. You can approve them right away; they are published when the
pull request is merged. See
[Agent proposals](../using-the-tracker/wiki.md#agent-proposals).

Every phase change can also move the task's own state — see
[Automatic state changes](../using-the-tracker/features.md#automatic-state-changes).

### Stage outputs in the activity feed

As the agent works, it posts each stage's result as a comment in the task
**activity feed**, so you can read its thinking before approving.

**Brainstorming** — the agent explores the problem and approaches.

![Brainstorming stage output](../../site/assets/img/stage-brainstorming.png)

**Design** — the chosen approach. The design comment may also attach one or more
HTML **UI mockups**, rendered inline as cards you can open in a full-window
sandboxed preview.

![Design stage output](../../site/assets/img/stage-design.png)

![Opened sandboxed mockup preview](../../site/assets/img/stage-mockup-preview.png)

**Implementation plan** — a step-by-step plan; it renders the unified diffs the
plan emits.

![Implementation plan stage output](../../site/assets/img/stage-plan.png)

**Implementation** — the agent applies the approved plan, commits, and pushes a
branch. It reports the result as the **pull-request-pushed** comment (shown under
[The pull request](#the-pull-request) below) — there is no separate step.

### Watching the agent think

The agent's thinking appears in the activity feed as a **Thinking** row, next to
the agent's avatar. There is one such row per stage.

While a stage runs, its row is open and scrolls itself: thinking and tool calls
are appended live — no refresh — next to the stage name, a working dot and the
elapsed time. Thinking reads as prose; each tool call sits in its own small card
with the argument that identifies it and a coloured icon for what the call did —
run a command, write, or read.

![An open Thinking row, the agent's reasoning for the brainstorming stage read as prose](../../site/assets/img/thinking-expanded.png)

Once the stage finishes, the row collapses under the comment that
stage produced; click the header to read it again. Collapsed it is a single line,
so it stays out of the way without needing a filter of its own.

![The same Thinking row collapsed to a single line under the stage name](../../site/assets/img/thinking-collapsed.png)

A stage that produces no comment — one that **failed**, or a brainstorm with
nothing to ask — still gets its own row at the end of the feed. That is usually
the thinking you most want after a failure.

Retrying a stage with **Continue** starts its thinking from scratch: the new
attempt never shows the failed one's thinking. Cancelling or restarting a run
keeps the last thoughts of whatever the stage had got through so far.

Not every model or gateway emits thinking. When none arrives, the row simply
shows the stage and the elapsed time — that is normal, not an error, and never a
stuck spinner. If the tracker is briefly unreachable the gateway retries the
batch, so a restart or a blip costs seconds of thinking rather than the rest of
the stage; when thinking is genuinely missing from the middle, the row says so.
A stage that hits the per-stage size limit keeps what it had and says the rest
was dropped, rather than ending without explanation. That limit is final for the
stage: nothing is recorded after the message, so what you read up to it is
continuous.

The gateway sends thinking as a stream of typed events — a thought, or a tool
call with its name and argument as separate fields — and the tracker stores and
replays exactly that. Nothing along the way renders the events into a line of
text that would have to be parsed back apart, so an agent whose thinking quotes
an arrow, a bracket or any other marker is shown as the prose it wrote. This is
the contract every gateway implements, not something specific to Goose.

**What is stored** is up to the instance admin, in **Admin → Settings → Keep the
full agent thinking**:

- **On** (default): the full thinking of each stage is kept and can be reopened
  from the Thinking row at any time.
- **Off**: only a ~1 KB tail — the last thing the agent was thinking about —
  is kept, and the row is labelled `last thoughts only`.

**Agent thinking kept per stage (KB)**, right below it, is the size limit a
stage may fill before the rest is dropped. The default is 1024 KB (1 MB) per
stage; it accepts 64 KB to 10240 KB. A stage that hits the limit keeps what it
had and the row says the rest was dropped.

Both settings apply to the next batch of thinking; no restart is needed.

![Admin application settings, with the thinking retention toggle and the per-stage KB limit](../../site/assets/img/admin-settings.png)

> Thinking text can quote source code and tool output, exactly like a design or
> plan comment. It is visible to every member of the project, so treat it with
> the same care — a secret the agent reads can surface here.

### Approving a stage

When a stage reaches `awaiting_approval` (the design and plan gates), review the
agent's output in the activity feed and click **Approve** to let it continue to
the next stage. This is the human-in-the-loop gate — nothing proceeds without
your sign-off. A [short workflow](#short-workflow-for-small-tasks) run has no
such gates.

When a design offers **more than one mockup**, there's no single Approve button
— each mockup card carries its own **Use & approve design** action. Click it on
the variant you want: that mockup is marked as chosen, the others are dimmed, and
the agent implements the one you picked. No need to comment or have the design
re-posted — the run advances straight to the next stage with your choice.

![Mockup cards in the design comment — one chosen, the other dimmed](../../site/assets/img/message-mockup.png)

### The pull request

After the implement stage the agent pushes a branch, and **the tracker opens the
PR/MR** for it through the project's [git integration](../using-the-tracker/git-integration.md)
(works for GitHub, GitLab and Gitea — the agent never runs `gh`). The run exposes
`prUrl`, `prId`, `prHostType`, and `branchName`. The task's pull request panel
links to the PR.

![Run in pr_open with the PR link](../../site/assets/img/run-pr.png)

### Reviewing the pull request

While the run sits in `pr_open`, commenting on the task starts another implement
attempt. Review it as a conversation rather than a form: the agent reads the whole
thread from the beginning, its own messages included, and treats the newest
instruction as the one that counts. Asking for something that departs from the
approved plan is normal and the agent keeps the change — it will say when a request
conflicts with an earlier decision instead of quietly reverting to the plan.

Not every comment produces a commit. When you ask a question, or the agent objects
to a request, it answers with a **review reply** comment and nothing is pushed.
Your next comment picks the conversation back up.

Every attempt pushes to the **same branch and PR** for as long as that PR stays
open, even days later or after the gateway cleaned up its working copy. A new
branch and PR are opened only once the previous PR was merged or closed.

### Surviving a restart

Agent work runs in the gateway, a separate process from the tracker. If the
**tracker restarts** (upgrade, outage) while a run is in progress, the run stays
in progress and the gateway keeps working. Its heartbeats resume as soon as the
tracker is back, and the stage output and pull request are recorded as usual. No
work is lost — you don't need to do anything.

If a gateway sends no heartbeat for 10 minutes, the tracker fails the stage. A
gateway that is still alive then stops the agent and frees its slot, so the next
stage is not stuck waiting behind work nobody will accept.

If the **gateway** itself restarts (its subprocess dies with it), the affected run
is failed and you resume it with **Continue** or **Restart**.

Because a reconciled run already carries its pull request, **Restart is refused on
a run that has a PR** — use **Continue** instead.

## Troubleshooting

| Symptom                                                           | Likely cause                                                                                                                                                   |
| ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Webhook returns `401`                                             | `TRACKER_TO_GATEWAY_TOKEN` mismatch between tracker and gateway                                                                                                |
| Tracker calls rejected                                            | Wrong/expired `GATEWAY_TO_TRACKER_TOKEN`                                                                                                                       |
| Run fails immediately with auth error                             | Wrong/missing provider credential — check the provider of each `MODELS` entry matches the keys you set ([connection options](./gateway.md#connection-options)) |
| Agent can't push                                                  | The project's git integration token lacks write on the repo, see [token scopes](../using-the-tracker/git-integration.md#token-scopes)                          |
| Run fails at the first stage with "no git integration"           | The project has no git integration; add one in **Project settings → Git integrations**                                                                       |
| Nothing happens after assigning                                   | Gateway not running / not reachable at the configured Gateway URL                                                                                              |
| Run briefly shows `failed` after a tracker restart, then recovers | Expected — the tracker reconciles the live gateway's completion automatically                                                                                  |
| **Restart** is refused / shows "use Continue"                     | The run already has an open PR — use **Continue**                                                                                                              |

See the [gateway verifying-a-deploy steps](./gateway.md#healthcheck--verifying-a-deploy)
to confirm the gateway side is healthy.
