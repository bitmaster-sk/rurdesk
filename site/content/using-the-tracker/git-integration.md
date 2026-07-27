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

| Field | Meaning | Example |
| --- | --- | --- |
| **Name** | Display label | `backend repo` |
| **Host type** | `github` · `gitlab` · `gitea` | `github` |
| **Base URL** | API root of the host | `https://api.github.com`, `https://gitlab.com`, or your self-hosted URL |
| **Repo path** | `owner/repo` path on the host | `your-org/your-repo` |
| **Access token** | Token used to open PRs/MRs and read their diffs/status | see scopes below |

Fill the form and save:

![Git integration create form](../../site/assets/img/git-integration-form.png)

A project can have **multiple integrations** (e.g. backend + frontend repos);
they're listed on the project's Git integrations settings page:

![Git integrations settings page](../../site/assets/img/git-integration-settings.png)

### Token scopes

The token both **reads** PR/MR diffs & status **and opens** the PR/MR for agent
runs, so it needs **write** access:

| Host | Token | Scope |
| --- | --- | --- |
| GitHub | fine-grained PAT | *Pull requests: write* + *Contents: read* (or classic `repo`) |
| GitLab | project/personal access token | `api` |
| Gitea | access token | `write:repository` |

> Read-only tokens (`read_api`, *Pull requests: read*, `read:repository`) are
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

![MR diff inline](../../site/assets/img/mr-diff.png)

## Permissions

| Action | Required role |
| --- | --- |
| View integrations, diffs, status | project **member** |
| Create / update / delete integrations | project **owner** |

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| Diff/status returns an error | Token expired or missing scope; base URL wrong (self-hosted hosts need the full API root) |
| Agent run fails at "open PR" | Token lacks **write** scope, or `repoPath`/`baseUrl` is wrong — the host rejected the create call |
| `403` on the settings page | You're a member, not an owner |
| Agent pushed a branch but no PR opened | No git integration on the project whose `repoPath` matches the pushed repo (the tracker can't tell which integration to use) |
