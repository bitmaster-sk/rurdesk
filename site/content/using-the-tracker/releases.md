---
title: Releases
description: Turn a tag into a version. Start a draft, check the tasks, write the release notes by hand or with an agent, and release when you are ready.
---

# Releases

A **release** turns a [tag](tags.md) into a version. Collect the tasks of the
next release under any tag, for example `next`. When you ship, the tag is
renamed to the version, say `v1.0.4`, and its closed tasks become the content of
the release. The release notes live on the release itself, and the
**Releases** page keeps the history.

![The Releases page: every version with its state, who released it and when, and the number of its tasks](../../site/assets/img/release-list.png)

A release starts as a **draft**. Nothing changes on the tag or the tasks while
you work on it; only **Release** does that, all at once.

## Starting a draft

Open **Releases** in the project menu and click **New release**, or run
*Release a version* from the command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>).
Pick the tag that holds the release and type the version name, then **Start the
draft**. A tag has at most one draft; when you pick a tag that already has one,
the page links to it instead.

The draft page leads you through three steps. Every change is saved on its own
after a moment, so you can leave and come back later; the Releases page lists
the draft first, with **Continue** to open it again.

1. **Version.** The name of the version. On release the tag is renamed to this
   name, so the tasks keep it. *Create an empty tag again* (on by default)
   creates a new tag with the old name and color, so you keep collecting the
   next release under the same name. Turn it off for a tag you only needed
   once.
2. **Tasks.** The closed tasks, the ones in a state marked *final*, grouped by
   task type. Only they go into the release notes. Every task opens in a new
   tab. When the tag has no closed task, or some of its tasks are not closed
   yet, a warning says so. A tag without a closed task can still be released.
   Tasks that are not closed are listed, and the page asks what to do with them
   on release:
   - **Move them** to the new tag (the default). Without a new tag, the tag is
     only removed from them.
   - **Release them too**, so they keep the version tag.
3. **Release notes.** While the notes are empty, the step asks how to write
   them: **Write it myself** or **Write with an agent** (see below). Writing
   them yourself opens the same editor as in the wiki, with a preview next to
   it. It starts with the closed tasks, one section per task type in the order
   of the types, tasks without a type last:

   ```markdown
   ## Feature

   - Dark mode ([#105](/project/3/issue/105))

   ## Bug

   - Login loops after a password change ([#98](/project/3/issue/98))
   ```

   Rewrite it for your readers. **Rewrite with an agent** above the editor
   lets an agent write it anew.

**Release v1.0.4** in the page header does everything at once: the rename, the
new tag and the moved tasks. It stays disabled, with the reason next to it,
until the version has a name and the release notes have some text. While
*Create an empty tag again* is on, the version needs a name other than the tag.
When the tag has no closed task, the confirmation says so. When something
fails, for example the version name is already taken by another tag or
release, nothing is changed and the draft stays.

Only **project owners** (and admins) work on drafts and release. Every member
sees the Releases page, the drafts with their tasks and the release notes; to
them a draft is read-only.

When the tag of a draft is deleted, the draft says so and cannot be released.
Delete it and start a new one from another tag.

## Letting an agent write the release notes

**Write with an agent** on the release notes step opens a form: pick the agent
and, if you want, change the skills, the model and the turn limit of the
*Release notes* stage. The model starts at the default model of the agent's
gateway. **Start writing** lets the agent write. No task is created and nothing
is posted to any task. The agent gets exactly the tasks of the release: the
closed ones and, when you chose to release the open tasks too, those as well.
It writes only about them and never touches code. A release with no task at all
has nothing to write about, so the agent stops with an error and you write the
notes yourself.

While the agent writes, the step shows its thinking live in place of the editor,
and **Stop** stops the agent. The other steps stay open, and the agent dock
shows the agent busy with the version. When the agent is done, its text lands in
the draft, replacing what was there, and the editor opens with it. Edit it like
your own. When the agent fails, the step says so and you can try again.

How the agent writes is up to the skills of the *Release notes* stage. The
builtin **Release notes style** skill is on by default. It fixes the structure:
a sentence or two that sum up the release, then *Highlights*, *Breaking
changes*, *Added*, *Changed*, *Removed* and *Fixed*, without the sections that
would be empty. It asks for plain words for the people who use the product,
facts from the tasks only, and the task numbers at the end of each bullet. See
[Agent skills](agent-skills.md).

## After the release

The release page shows who released it and when, the number of tasks it
contained and its release notes. Below the notes, **Tasks in this release**
lists every task that carries the version tag, with its number, type, title,
tags, severity and state; a title opens the task. **Edit release notes** opens
the same editor with a preview next to it; **Save** stores the new text, which
cannot be empty. The notes are not copied to the wiki.

![A released version: who released it and when, its release notes, and the tasks in this release below them](../../site/assets/img/release-notes.png)

Changes made by other members appear without a reload.

## Deleting a release

The bin in the page header deletes the release.

- A **draft** simply goes away. The tag and its tasks were never touched, so
  they stay as they are. An agent still writing the notes is stopped.
- For a **released** version the dialog asks what to do with the version tag:
  - **Keep it as it is.** The tag and its tasks stay.
  - **Move the tasks to another tag and delete it.**
  - **Move the tasks to another tag and keep it**, now empty.

  The tag is never renamed back: a tag with the old name may exist again by
  then.
