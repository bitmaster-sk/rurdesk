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
tracker. It is also the repository agents work in: the tracker hands the
repository, its base branch and the token to the
[gateway](../agentic-workflow/gateway.md#repositories) with every stage.

> **The tracker opens the PR itself.** When an agent finishes implementing, it
> only pushes its branch; the tracker then opens the PR/MR through this
> integration's API. That's why the access token needs **write** permission
> (see [Token scopes](#token-scopes)), not just read.

> **One integration per project.** A project connects to exactly one
> repository, and agent runs on its tasks work there. Delete the integration to
> connect a different repository.

## Add an integration

**Project settings → Git integrations** (project **owner** only):

<!-- stack-table -->

| Field            | Meaning                                                | Example                                                             |
| ---------------- | ------------------------------------------------------ | ------------------------------------------------------------------- |
| **Name**         | Display label                                          | `backend repo`                                                      |
| **Host type**    | `github` · `gitlab` · `gitea`                          | `github`                                                            |
| **Base URL**     | Web URL of the host, prefilled for the cloud hosts     | `https://github.com`, `https://gitlab.com`, or your self-hosted URL |
| **Repo path**    | `owner/repo` path on the host                          | `your-org/your-repo`                                                |
| **Base branch**  | Optional. Agents branch from it and open PRs/MRs against it; empty means the repository's default branch, or `main` for an empty repository | `develop` |
| **Access token** | Token used to open PRs/MRs, read their diffs/status and push agent branches | see scopes below                                       |

Picking a host type fills in its cloud URL (`https://github.com`,
`https://gitlab.com`, `https://gitea.com`). A URL you typed yourself is kept.

Fill the form and save:

![Git integration create form](../../site/assets/img/git-integration-form.png)

The integration is listed on the project's Git integrations settings page. While
it exists, **New integration** is disabled:

![Git integrations settings page](../../site/assets/img/git-integration-settings.png)

### Token scopes

The token **reads** PR/MR diffs & status, **reads file contents** for diff expansion,
**opens** the PR/MR for agent runs, and the agent uses it to **clone and push** its
branch, so it needs **write** access to both pull requests and contents:

| Host   | Token                         | Scope                                                                      |
| ------ | ----------------------------- | -------------------------------------------------------------------------- |
| GitHub | fine-grained PAT              | _Pull requests: write_ + _Contents: read & write_ (or classic `repo`)      |
| GitLab | project/personal access token | `api` (a project token needs the **Developer** role or higher)             |
| Gitea  | access token                  | `write:repository`                                                         |

> Read-only tokens (`read_api`, _Pull requests: read_, `read:repository`) are
> not enough: diffs and status will load, but pushing the branch or opening a
> PR/MR fails and the agent run is marked failed with the host's error message.

> The agent sees the token of the run it works on, so scope it to this one
> repository. The gateway never writes it to disk, see
> [Repositories](../agentic-workflow/gateway.md#repositories).

### Check the connection

**Check connection** in the form asks the host whether the integration works,
without saving it. **Save** runs the same check first. If the check fails, the
form shows why and the button changes to **Save anyway**, so you can still
store an integration the tracker cannot reach right now.

| Result               | Meaning                                                                        |
| -------------------- | ------------------------------------------------------------------------------ |
| Connected            | The token works and can open PRs/MRs. The repository's default branch is shown |
| Read-only            | The token works but cannot write; opening PRs/MRs for agent runs will fail     |
| Token rejected       | The host refused the token: wrong, expired or revoked                          |
| Repository not found | Wrong repo path, or the token has no access to that repository                 |
| Host unreachable     | No usable answer from the host API; usually a wrong base URL                   |

When you edit an integration and leave the token blank, the check uses the
stored token.

> The check reads what the host reports: write access on the repository
> (GitLab: _Developer_ or higher), plus the `repo` scope for classic GitHub
> tokens and the `api` scope for GitLab tokens. A GitHub **fine-grained** PAT
> does not expose its permissions, so the check cannot tell whether it has
> _Pull requests: write_.

### Token security

- Tokens are stored **encrypted at rest** (AES-GCM with a per-row nonce) and
  are **never returned by the API** — responses only carry a `hasToken` flag.
- Updating an integration without entering a token **keeps the existing one**;
  enter a new value only to rotate it.

## What you get

- **MR/PR diff view** — the diff rendered in the app (cached server-side,
  so repeated views don't re-hit the host API).
- **Expand hidden lines** — the arrow buttons on a hunk header (and below the last
  hunk) show the unchanged lines around a change: 20 at a time, or the whole gap
  when it is short. The file is read from the host once per commit, so further
  clicks are instant. Added and deleted files are already shown in full.
- **Show full file** — the icon next to a file's +/- counts shows every
  unchanged line of that file at once. Click it again to go back to the
  changes only.
- **MR/PR status** — open / merged / closed state next to the link. It updates
  on the open task by itself — CI result, approval and the final merge — so you
  don't have to reload the page.
- **Agent run cards** link the PR and show its diff inline, so the
  review-and-approve loop stays in one place.
- **Manual linking** — on any task, the PR panel lets you **link an existing
  pull/merge request** yourself (no agent run required). Once linked, the
  panel shows the same status pill, diff, and a **View PR** link as it would
  for an agent-opened PR — task detail no longer distinguishes the two once a
  PR is linked. See [Automatic state changes](./features.md#automatic-state-changes)
  for what happens to the task's state when a manually linked PR is merged.

![The link-pull-request dialog: pick the git integration, then give the request's number](../../site/assets/img/pr-link-picker.png)

![MR diff inline](../../site/assets/img/mr-diff.png)

### CI status

Next to the PR/MR state the tracker shows the CI result for the **head commit**
of the change request. All the jobs of that commit are reduced to one label:

<!-- stack-table -->

| Label           | When                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------ |
| **CI failed**   | at least one job failed, timed out, errored, or needs manual action                        |
| **CI pending**  | nothing failed and at least one job is still running or queued                             |
| **CI canceled** | nothing failed or running, and at least one job was canceled                               |
| **CI passed**   | nothing failed, running, or canceled, and at least one job succeeded or reported a warning |
| **CI skipped**  | every job was skipped                                                                      |
| _no label_      | the host reported no CI for this commit                                                    |

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

The **Approved** tag appears when at least one person has approved the change
request. It is read per host: GitHub and Gitea from the PR's reviews, GitLab from
the people listed on the merge request's approvals endpoint (available on **all
GitLab tiers including Free** — only approval _rules_ are Premium). A GitLab merge
request that meets its approval rules but that nobody approved shows no tag.

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
| Agent run fails right at the start     | The project has no git integration, so the tracker does not know which repository to send to the gateway                    |
| Agent can't push its branch            | The token lacks _Contents: write_ (GitHub) or write access to the repository                                                |
| **New integration** is disabled        | The project already has its one integration                                                                                  |
