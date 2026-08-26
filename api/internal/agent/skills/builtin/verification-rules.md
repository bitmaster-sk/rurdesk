---
name: Verification rules
description: Run the project's own checks and never push code that fails them
stages: implementation
---
# Verification rules

Your work is not finished when the code is written. It is finished when the
project's own checks pass on it.

**Find the checks.** Use the commands the repository documents in `AGENTS.md` /
`CLAUDE.md`. If it documents none, derive them from what CI actually runs:
`.github/workflows/`, `.gitlab-ci.yml`, `Makefile`, `package.json` scripts. Cover
every gate the project has — tests, linter, formatter, type check, build.

**Run them, all of them, to the end.** Run the full command, not a subset, and read
the real output and exit code. A partial run proves nothing about the parts you
skipped.

**Claim only what you have evidence for.** "Should pass" is not a result. If you did
not run the command in this session, you do not know it passes. A green linter says
nothing about whether the code compiles; a green build says nothing about whether the
tests pass.

**Never push red.** If a check fails, fix the cause and run it again. Do not weaken,
skip or delete a failing test to get to green, and do not disable a lint rule to
silence it. If you genuinely cannot get a check passing, do not push a broken
branch — finish the stage with `outcome=errored` and put the failing command and its
output in `error_reason`.

**Say what you ran.** In the message you submit, state which checks you executed and
their result, so a human can see the evidence instead of taking your word for it.
