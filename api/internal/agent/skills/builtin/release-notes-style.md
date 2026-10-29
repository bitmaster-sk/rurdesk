---
name: Release notes style
description: The fixed structure of release notes, written from the tasks of the release for the people who use the product
stages: release_notes
---
# Release notes style

**Reader.** Write for the people who use the product, not for the team that built it.
Say what changed for them and why it matters. Leave out internal names, branch names,
file paths, ticket jargon and anything only the team would understand.

**Structure.** Always use this structure, in this order. The task types you are given only
help you sort the tasks; they are not sections.

```markdown
One or two sentences that sum up the release: the two or three changes that matter most.

### Highlights

- **Short name of the change** — what the user can do now and why it matters, in one or
  two sentences.

### Breaking changes

- **What stops working as before.** What the user has to do about it.

### Added

- One bullet per new feature or option. (#12)

### Changed

- One bullet per change to something that already existed. (#14, #15)

### Removed

- One bullet per feature or option that is gone. (#16)

### Fixed

- One bullet per bug: what went wrong, said so a user recognizes it. (#18)
```

- **Highlights** names the three to five changes a user would care about most, with a bold
  name and a dash. The same change also gets its own bullet under Added, Changed or Fixed.
- **Breaking changes** lists only what forces the user to act: something removed, renamed
  or behaving differently in a way they will notice.
- Leave out a section that would be empty. Never write "None" or "No changes".
- Do not add sections of your own, such as Database or Upgrading. You see only the tasks,
  so you cannot know what an upgrade needs.

**Bullets.** One bullet per change, starting with what the user can now do or what no
longer goes wrong. Merge tasks that describe the same change into one bullet. End each
bullet in Added, Changed, Removed and Fixed with the numbers of its tasks in parentheses,
for example (#104), so the team can trace it.

**Facts only.** Write only about the tasks of the release you are given, and describe only what they say. Do not invent features, numbers or
dates. When a task is unclear, read it with `get_issue` before you write about it, and
leave it out rather than guess.

**Language.** Write in the language the project's tasks are written in. No closing
remarks.
