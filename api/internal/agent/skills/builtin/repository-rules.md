---
name: Repository rules
description: Read the repository's own agent instructions before changing anything
stages: design, implementation_plan, implementation
---
# Repository rules

Before you plan or change anything, read the instructions the repository ships for
agents. Look for them in this order and read the first ones that exist:

1. `AGENTS.md` in the repository root (the open standard most tools read).
2. `CLAUDE.md`, `GEMINI.md` or `.cursor/rules/` — tool-specific equivalents.
3. Nested `AGENTS.md` files closer to the code you are touching. The nearest file
   to a changed file wins where two disagree.
4. `CONTRIBUTING.md` for conventions aimed at humans that still bind you.

Treat what you find as binding: build and test commands, directory layout, naming,
formatting, commit and PR rules. Where those instructions conflict with your own
habits, the repository wins.

If none of those files exist, do not guess the project's conventions — derive them
from what the repository actually does: CI configuration (`.github/workflows/`,
`.gitlab-ci.yml`), `Makefile` targets, `package.json` scripts, and the style of the
files next to the ones you are editing.
