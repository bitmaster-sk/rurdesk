---
title: Tags
description: Free labels on tasks — create them per project, put them on tasks while you type, filter by them in every view and through the API.
---

# Tags

A **tag** is a free label you put on a task. Unlike the state, it is no step in
a workflow, and unlike the type, a task can carry any number of them. Use tags
for everything that cuts across the workflow: the area of the code (`gateway`,
`wiki`), the customer who asked for it, a flag like `blocked-external`, or the
release a finished task belongs to.

Tags belong to a project, the same way states, severities and types do. Each
tag has a name and a color. A name is unique within the project regardless of
letter case, so `bug` and `Bug` cannot both exist.

## Managing tags

Open **Project settings** and find the *Tags* panel. *New tag* asks for a name
and a color from a palette of ten. The table lists every tag with the number of
tasks that carry it; drag a row to change the order tags appear in.

![The tags panel in project settings: each tag with its task count, drag handles, and edit and delete buttons](../../site/assets/img/project-tags.png)

- **Rename** a tag with the pencil. Every task keeps the tag and shows the new
  name right away, no task has to be touched.
- **Delete** a tag with the bin. The dialog tells you how many tasks carry it.
  Deleting removes the tag from those tasks; the tasks themselves stay.

Only **project owners** (and admins) create, rename, recolor and delete tags.
Every member who can edit a task can put existing tags on it and take them off.

## Tagging a task

The task detail has a *Tags* field right under the title. Type into it
directly:

![The tags field of a task with "ne" typed: "next" completed in grey, matching tags listed below, and an offer to create "ne" with a color](../../site/assets/img/tag-input.png)

- Matching tags appear under the field as you type, tags whose name starts with
  your text first. The first match is completed in grey right in the field.
- <kbd>Tab</kbd> or <kbd>Enter</kbd> puts the highlighted tag on the task,
  <kbd>↑</kbd>/<kbd>↓</kbd> pick another one, <kbd>Esc</kbd> closes the list.
- <kbd>Backspace</kbd> in an empty field removes the last tag; the × on a tag
  removes that one.
- When no tag has the name you typed, the last row offers to **create** it,
  together with a color — the first color the project does not use yet is
  preselected, <kbd>←</kbd>/<kbd>→</kbd> or a click pick another. Members who
  may not create tags see a short note instead.

Every change is saved immediately, like the other fields of the detail.

## Where tags show

Tags appear in the **task table**, after the task title, and on **Kanban
cards**, between the title and the type and state. At most three are shown,
the rest as *+N*; point at it to see the other tags. The Calendar and Gantt views do
not show tags; an event or a short bar has no room for them.

![A table row with three tags and +1; pointing at +1 shows the hidden tag](../../site/assets/img/issue-tags-more.png)

Tags are drawn on purpose with a light outline in their color and an italic
name. The state and severity already use solid color, so tags stay quieter and
cannot be mistaken for either.

## Filtering

The filter panel has a *Tags* field in every view — table, Kanban, Calendar and
Gantt. A task matches when it carries **at least one** of the selected tags.
*Display tasks without tags* adds the untagged tasks to the result. The tag
filter is saved into a [saved view](saved-views.md) like any other filter.

## Release contents with tags

A tag can collect what goes into the next release. Put a tag such as `next` on
tasks as you plan them. When you ship, [release](releases.md) the tag: start a
draft, write the release notes from the closed tasks, and release it. The tag is
renamed to the version, a new empty `next` is created and unfinished tasks move
to it.

## API and agents

Tasks carry their tags as `idsTag`, a list of tag ids; the tag list itself
(`GET /api/private/tag`) maps ids to names and colors. Over
[MCP](features.md#mcp-server):

| Tool | What it does |
| --- | --- |
| `list_tags` | Tags of the projects you can see, with their task counts |
| `create_tag`, `update_tag`, `delete_tag` | Manage tags; project owners only |
| `list_issues` with `tag_ids` | Tasks with at least one of the tags; `tag_unset` adds untagged tasks |
| `create_issue`, `update_issue` with `tag_ids` | Set the tags of a task; on update, an empty list removes all tags and leaving the argument out keeps them |
| `bulk_update_issues` with `add_tag_ids`, `remove_tag_ids` | Add or remove tags on many tasks at once; their other tags stay |
