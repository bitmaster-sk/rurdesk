---
title: Upgrading
description: What to change when you move to a new release, version by version.
---

# Upgrading

Most releases need nothing beyond pulling the new images, see
[Updating](./installation.md#updating). This page lists the releases that need
more. Go through every section between your current version and the one you
move to, oldest first.

## Upgrading to 1.6.0

The gateway now takes the repository, the base branch and the git token of a
run from the project's [git integration](../using-the-tracker/git-integration.md),
not from its own environment.

Update the tracker and every gateway together. A 1.6.0 gateway fails every stage
of an older tracker with `tracker_missing_repo`, because that tracker does not
send the repository.

1. Add a git integration to every project the agent works on, and give its token
   the [scopes](../using-the-tracker/git-integration.md#token-scopes) for both
   pull requests and pushing. To keep branching from something other than the
   default branch, set **Base branch** to the value of `REPO_BRANCH_BASE`.
2. Remove `REPO_URL`, `GIT_ACCESS_TOKEN` and `REPO_BRANCH_BASE` from the
   gateway's environment. The gateway ignores them and logs a warning while they
   are set.
3. Start the gateway. It deletes the old clone (`/worktrees/<repository>`), whose
   git config held the token. Open runs continue on their branch on the remote.
