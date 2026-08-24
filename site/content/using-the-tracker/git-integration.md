---
title: Git Integration
description: Connect GitHub, GitLab, or Gitea repositories to a project — PR/MR diffs and status inline.
---

# Git Integration

A **git integration** connects a project to a repository on **GitHub**,
**GitLab**, or **Gitea** (cloud or self-hosted). Once connected, the tracker
talks to the host's API to **open pull/merge requests for agent runs** and to
show **PR/MR diffs and status** right inside the app — most importantly on
[agent run](./agents.md) cards, where the PR is reviewable without leaving the
tracker.

> **The tracker opens the PR itself.** When an agent finishes implementing, it
> only pushes its branch; the tracker then opens the PR/MR through this
> integration's API. That's why the access token needs **write** permission
> (see [Token scopes](#token-scopes)), not just read.

> Don't confuse this with the gateway's
> [`REPO_URL`/`GIT_ACCESS_TOKEN`](../agentic-workflow/gateway.md#gitaccesstoken-scopes),
> which let the **agent** clone and push. The git integration is how the **tracker
> UI** reads PRs/MRs back from the host. For the full agent loop you typically
> configure both against the same repository.

## Add an integration

**Project settings → Git integrations** (project **owner** only):

<!-- stack-table -->

| Field            | Meaning                                                | Example                                                                 |
| ---------------- | ------------------------------------------------------ | ----------------------------------------------------------------------- |
| **Name**         | Display label                                          | `backend repo`                                                          |
| **Host type**    | `github` · `gitlab` · `gitea`                          | `github`                                                                |
| **Base URL**     | API root of the host                                   | `https://api.github.com`, `https://gitlab.com`, or your self-hosted URL |
| **Repo path**    | `owner/repo` path on the host                          | `your-org/your-repo`                                                    |
| **Access token** | Token used to open PRs/MRs and read their diffs/status | see scopes below                                                        |

Fill the form and save:

![Git integration create form](../../site/assets/img/git-integration-form.png)

A project can have **multiple integrations** (e.g. backend + frontend repos);
they're listed on the project's Git integrations settings page:

![Git integrations settings page](../../site/assets/img/git-integration-settings.png)

### Token scopes

The token both **reads** PR/MR diffs & status **and opens** the PR/MR for agent
runs, so it needs **write** access:

| Host   | Token                         | Scope                                                         |
| ------ | ----------------------------- | ------------------------------------------------------------- |
| GitHub | fine-grained PAT              | _Pull requests: write_ + _Contents: read_ (or classic `repo`) |
| GitLab | project/personal access token | `api`                                                         |
| Gitea  | access token                  | `write:repository`                                            |

> Read-only tokens (`read_api`, _Pull requests: read_, `read:repository`) are
> not enough: diffs and status will load, but opening a PR/MR fails and the
> agent run is marked failed with the host's error message.

### Token security

- Tokens are stored **encrypted at rest** (AES-GCM with a per-row nonce) and
  are **never returned by the API** — responses only carry a `hasToken` flag.
- Updating an integration without entering a token **keeps the existing one**;
  enter a new value only to rotate it.

## What you get

- **MR/PR diff view** — the full diff rendered in the app (cached server-side,
  so repeated views don't re-hit the host API).
- **MR/PR status** — open / merged / closed state next to the link.
- **Agent run cards** link the PR and show its diff inline, so the
  review-and-approve loop stays in one place.
- **Manual linking** — on any task, the PR panel lets you **link an existing
  pull/merge request** yourself (no agent run required). Once linked, the
  panel shows the same status pill, diff, and a **View PR** link as it would
  for an agent-opened PR — task detail no longer distinguishes the two once a
  PR is linked. See [Automatic state changes](./features.md#automatic-state-changes)
  for what happens to the task's state when a manually linked PR is merged.

![MR diff inline](../../site/assets/img/mr-diff.png)

### CI status

Next to the PR/MR state the tracker shows the CI result for the **head commit**
of the change request. All the jobs of that commit are reduced to one label:

<!-- stack-table -->

| Label           | When                                                                 |
| --------------- | -------------------------------------------------------------------- |
| **CI failed**   | at least one job failed, timed out, errored, or needs manual action  |
| **CI pending**  | nothing failed and at least one job is still running or queued       |
| **CI canceled** | nothing failed or running, and at least one job was canceled         |
| **CI passed**   | nothing failed, running, or canceled, and at least one job succeeded |
| **CI skipped**  | every job was skipped                                                |
| _no label_      | the host reported no CI for this commit                              |

A failure always wins, so a long-running matrix can never hide a job that has
already failed. If the repository has no CI at all, **no label is shown** — the
tracker doesn't claim a status it doesn't have.

Where the result is read from:

| Host   | Source                                                                       |
| ------ | ---------------------------------------------------------------------------- |
| GitHub | check runs (GitHub Actions), falling back to commit statuses for external CI |
| GitLab | the merge request's latest pipeline                                          |
| Gitea  | commit statuses (where Gitea Actions reports)                                |

### Approvals

The **Approved** tag appears when a review has approved the change request. It
is read per host: GitHub and Gitea from the PR's reviews, GitLab from the merge
request's approvals endpoint (available on **all GitLab tiers including Free** —
only approval _rules_ are Premium).

## Permissions

| Action                                | Required role      |
| ------------------------------------- | ------------------ |
| View integrations, diffs, status      | project **member** |
| Create / update / delete integrations | project **owner**  |

## Troubleshooting

| Symptom                                | Likely cause                                                                                                                 |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Diff/status returns an error           | Token expired or missing scope; base URL wrong (self-hosted hosts need the full API root)                                    |
| Agent run fails at "open PR"           | Token lacks **write** scope, or `repoPath`/`baseUrl` is wrong — the host rejected the create call                            |
| `403` on the settings page             | You're a member, not an owner                                                                                                |
| Agent pushed a branch but no PR opened | No git integration on the project whose `repoPath` matches the pushed repo (the tracker can't tell which integration to use) |
