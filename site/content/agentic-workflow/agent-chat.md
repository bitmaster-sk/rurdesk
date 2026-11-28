---
title: Agent chat
description: Ask an agent about your projects, read-only, and turn an answer into a task.
---

# Agent chat

Agent chat is a private conversation between you and one of your
[agents](./agents.md). You ask about a project, its tasks, its wiki or its code,
and the agent answers. The agent only **reads**: it cannot change tasks, the
wiki or the code. When an answer is worth acting on, you turn it into a task
yourself, see [Create task](#create-task).

![Agent chat: the conversation list and an answer about the current sprint](../../site/assets/img/agent-chat-overview.png)

Open it from **Agent chat** in the left menu. Its count shows how many
conversations have an answer you have not read yet.

The panel next to the conversation lists all your conversations: the pinned
ones first, then those of the current project under its name, then the other
projects. A dot marks an unread answer; the others show their date. Search the
list by title with the field on top, start a new conversation with **+**, and
hide the panel when you want the conversation alone (see
[Navigation](../using-the-tracker/features.md#navigation)).

## Starting a conversation

**New conversation** asks for:

- **Projects**, optional. The first one is the **main project**: the agent reads
  its code, tasks and wiki. Further projects are **context**: the agent reads
  their tasks and wiki, but not their code. Without any project the agent
  answers from its own knowledge only.
- **Agent**. You get the agents that are members of the main project. The agent
  cannot be changed later; to ask another one, start a new conversation.
- **Model**. The gateway's default unless you pick another one. You can change
  it later in the header, except while the agent is answering.

![New conversation form](../../site/assets/img/agent-chat-new.png)

After the first message the main project is fixed. Context projects can still be
added and removed in the header. The agent reads only what **both you and the
agent** may read in those projects.

## Asking

- Your message waits behind the step the agent is working on, but **ahead of
  its queued task runs**, so you do not wait for its whole backlog. While it
  waits, the header shows its position in the queue.
- While the agent answers, you see its **thinking** and tool calls live. Later,
  the **Thinking** pill on the answer shows them again.
- A failed answer ends with an error line and a **Retry** button.
- The header shows the agent, its state, the tokens and the cost of the
  conversation, the model and the projects.

![The agent answering, with its thinking live](../../site/assets/img/agent-chat-live-thinking.png)

## Create task

Every answer has **Create task**. The agent turns that answer into a task draft
and shows it as a card in the conversation.

![A task draft in the conversation](../../site/assets/img/agent-chat-task-draft.png)

**Edit and save** opens the draft with its title, description and project.
Saving creates an ordinary task with the project's defaults and a link back to
the conversation. No agent starts working on it; you decide who does and when.

![Saving the task draft](../../site/assets/img/agent-chat-task-dialog.png)

## Pinning and deleting

The **⋮** button on a conversation in the list offers **Pin to top** and
**Delete**. A pinned conversation stays at the top of the list. **Delete**
removes it for good; an answer in progress is cancelled.

## Privacy

- Your conversations, their thinking and their cost are visible to **you only**,
  not to project members and not to administrators.
- Chats do not appear in a project's run lists. Other users see the agent only
  as "busy".
- Tokens spent in chats still count in the agent's 7-day total.

## For administrators

Chat needs no extra setup: every agent with a gateway can chat, with Claude Code
or Goose. Per answer the agent gets a short-lived token that can only read the
conversation's projects; its own API key is not used. How the gateway runs a
chat (tools, worktree, environment) is in
[Gateway → The chat stage](./gateway.md#the-chat-stage).

A message lets an agent spend tokens before the next task run starts. On an
agent that many people chat with, steady chat traffic delays its queued task
runs.

## Known limitations

- **Goose is read-only by instruction only.** It keeps its shell, so it could
  change files in its worktree (they are reset before the next answer) or the
  git config of the gateway's clone. Claude Code chat has no such tools.
- **The gateway's files are not limited to the conversation.** Claude Code's
  `Read`, `Grep` and `Glob` reach any file the gateway process can read,
  including clones of projects the user cannot see. Give one gateway only
  projects whose users may see each other's code.
- **Same OS user.** The agent runs as the gateway's OS user and could read its
  environment. The gateway removes its own secrets from the agent's environment;
  still, run it only with credentials you accept an agent seeing.
- **Instance wiki.** A chat on a project can also read the instance wiki.
- **Required custom fields.** A draft for a project with required custom fields
  cannot be saved from the dialog; create the task in the project instead.
- **Failed draft.** When a draft fails, Retry answers normally instead of drafting;
  press **Create task** again.
