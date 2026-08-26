---
name: Testing rules
description: Write the failing test first, then the code that makes it pass
---
# Testing rules

Write the test before the implementation, for every feature and every bug fix.

1. **Red** — write a test that describes the behavior you are about to add, and run
   it. Watch it fail, and check that it fails for the reason you expect. A test you
   never saw fail may be testing nothing at all.
2. **Green** — write the smallest amount of code that makes it pass. Resist adding
   anything the test does not demand.
3. **Refactor** — clean up the code you just wrote with the test still green.

For a bug fix, the first step is a test that reproduces the bug. If you cannot make
a test fail the way the bug does, you have not understood the bug yet.

Test behavior, not internals: assert what the code does through its public surface,
not how it does it inside. A test that breaks when you rename a private helper is
noise; a test that breaks when the behavior changes is the point.

Keep tests deterministic — no sleeps, no dependence on wall-clock time or on the
order tests run in, no shared mutable state between tests.
