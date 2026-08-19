---
title: Agents
description: Add a bot, wire a gateway, and run the AI agent workflow.
---

# Agents

This page covers the human side of the **AI Agent workflow**: creating a bot,
connecting it to a [gateway](./gateway.md), starting a run on a task, and
approving stages.

> Prerequisite: a running [gateway](./gateway.md) container (Goose, pointed at
> your chosen provider) that you can reach from the API over the compose network.

## Concepts

| Term | Meaning |
| --- | --- |
| **Bot user** | A non-human user account the agent acts as. Its writes are attributed to the bot (provenance). |
| **Gateway → Tracker token** | The bot's API token; the gateway sends it as `GATEWAY_TO_TRACKER_TOKEN` on every tracker call. |
| **Bot Gateway** | The bot's single gateway record (the Goose gateway's URL). Holds the Tracker → Gateway token the tracker signs webhooks with. |
| **Agent run** | One execution against one task: moves through stages, ends in a PR (or failure). |
| **Stage** | A phase of work: brainstorm → design → plan → implement. Some are skippable. |

## 1. Create a bot user

In **Administration → Users** click **Create user**, toggle **Bot**, give it a
name, and enter its **Gateway URL** — the Goose gateway's base URL, e.g.
`http://gateway-goose-qwen-cloud:9090`. Each bot has exactly **one** gateway and
can work on any project it is a member of.
More detail in [User management](./user-management.md#bots).

![Creating a bot](../../site/assets/img/bot-create.png)

## 2. Copy the bot's credentials

On creation the bot's **credentials dialog** opens with **both one-time tokens**.
Copy each immediately and set it in the gateway container's environment:

| Token | Gateway env var |
| --- | --- |
| **Gateway → Tracker token** | `GATEWAY_TO_TRACKER_TOKEN` — the gateway sends it on every tracker call |
| **Tracker → Gateway token** | `TRACKER_TO_GATEWAY_TOKEN` — the tracker signs each webhook with it |

Each token is shown **only once**; the dialog also records the bot's Gateway URL.
If you lose a token, reopen the dialog (key icon on the bot row) and use
**Regenerate token**, then reconfigure the gateway container.

The token row scrolls horizontally, so a long token stays fully readable and can
be selected by hand. **Copy** uses the browser clipboard API, which browsers
expose only over HTTPS (or `localhost`); on a plain-HTTP deployment it falls back
to a legacy copy path, and if the browser blocks that too the dialog says so —
select the token in the row and copy it manually.

![Bot credentials dialog — the one-time Gateway → Tracker and Tracker → Gateway tokens](../../site/assets/img/bot-keys.png)

> The tracker signs each `stage_execute` webhook with the Tracker → Gateway
> token; the gateway verifies the `X-Tracker-Signature` and rejects mismatches
> with `401`. The token must therefore be identical on both sides.

## 3. Start a run on a task

Assign a task to the bot (or trigger the agent action on the task). The API
enqueues a run and fires a `stage_execute` webhook at the gateway. The gateway
checks out a per-run worktree and starts the agent. From there the run drives
itself through the stages — assign, approve the gated stages, and it ends in a
PR:

![The agent workflow end to end](../../site/assets/img/agent-workflow.webm)

## 4. The run lifecycle

A run advances through **phases**:

| Phase | Meaning |
| --- | --- |
| `queued` | Waiting for a free slot (`MAX_CONCURRENT`) |
| `in_progress` | Agent is actively working a stage |
| `awaiting_input` | Agent needs clarification from a human |
| `awaiting_approval` | A stage finished and waits for human sign-off |
| `pr_open` | A pull/merge request has been opened |
| `done` | Run completed |
| `failed` | Run failed (see `errorMessage`) |
| `cancelled` | Run was cancelled |

Each **stage** carries its own status: `pending`, `active`, `done`,
`awaiting_approval`, `failed`, or `skipped`. The run card shows stage progress,
which bot executed each stage (provenance), and timestamps.

![Agent run card — stage timeline and phase badge](../../site/assets/img/run-card.png)

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

### Approving a stage

When a stage reaches `awaiting_approval` (the design and plan gates), review the
agent's output in the activity feed and click **Approve** to let it continue to
the next stage. This is the human-in-the-loop gate — nothing proceeds without
your sign-off.

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
`prUrl`, `prId`, `prHostType`, and `branchName` — the card links straight to the
PR for review.

![Run in pr_open with the PR link](../../site/assets/img/run-pr.png)

### Surviving a restart

Agent work runs in the gateway, a separate process from the tracker. If the
**tracker restarts** (upgrade, outage) while a run is in progress, the run is
briefly marked `failed` — but the gateway keeps working, and when it reports the
finished stage the tracker **reconciles it automatically**: the stage output and
the pull request are recorded and the run resumes its normal phase. No work is
lost and no duplicate PR is opened — you don't need to do anything.

If the **gateway** itself restarts (its subprocess dies with it), the affected run
is failed and you resume it with **Continue** or **Restart**.

Because a reconciled run already carries its pull request, **Restart is refused on
a run that has a PR** — use **Continue** instead (restarting would push a new branch
and open a duplicate PR).

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| Webhook returns `401` | `TRACKER_TO_GATEWAY_TOKEN` mismatch between tracker and gateway |
| Tracker calls rejected | Wrong/expired `GATEWAY_TO_TRACKER_TOKEN` |
| Run fails immediately with auth error | Wrong/missing provider credential — check `GOOSE_PROVIDER` matches the keys you set ([connection options](./gateway.md#connection-options)) |
| Agent can't push | `GIT_ACCESS_TOKEN` lacks write on the repo, or `REPO_URL` wrong |
| Nothing happens after assigning | Gateway not running / not reachable at the configured Gateway URL |
| Run briefly shows `failed` after a tracker restart, then recovers | Expected — the tracker reconciles the live gateway's completion automatically |
| **Restart** is refused / shows "use Continue" | The run already has an open PR — use **Continue** (Restart would open a duplicate PR) |

See the [gateway verifying-a-deploy steps](./gateway.md#healthcheck--verifying-a-deploy)
to confirm the gateway side is healthy.
