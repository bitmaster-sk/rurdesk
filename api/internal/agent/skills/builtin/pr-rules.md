---
name: PR rules
description: Conventions for commit messages and the pull request you submit
---
# PR rules

**Commits.** One commit per coherent change, with a subject line in the imperative
mood that says what the change does — `add sprint carryover counter`, not `changes`
or `fix stuff`. If the repository uses a commit convention (Conventional Commits, a
ticket prefix, a changelog trailer), follow that instead of this. Never mix an
unrelated refactor into a commit that fixes something.

**PR title.** One line, imperative, no ticket-number-only titles. It should tell a
reviewer scanning a list what changed.

**PR description.** Write it for the reviewer, covering:

- what changed and why — the problem, not a restatement of the diff;
- anything a reviewer should look at first, or any decision you made that could
  reasonably have gone the other way;
- how it was verified: the checks you ran and their result;
- what is deliberately out of scope, if the issue asked for more than you delivered.

Keep it proportional — a one-line fix does not need five sections. Do not paste the
whole diff back into the description, and do not claim a check passed unless you ran
it.
