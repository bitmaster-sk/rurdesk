---
title: Custom fields
description: Define your own typed fields per project, fill them in on a task, and let the API and agents read and write them.
---

# Custom fields

A **custom field** is a typed field you define yourself, once per project. Every
task in that project then carries it, next to the built-in fields like state,
severity and assignee.

Use one whenever your process tracks something the tracker does not model:
a customer name, an impact score, a review date, a "needs security review" flag.

## Creating a field

Open **Project settings** and find the *Custom fields* panel. *New field* asks
for:

- **Name** — what people see on the task.
- **Key** — how the API and agents refer to the field. It fills itself from the
  name and is locked; see *The key* below.
- **Type** — one of five, see below.
- **Default value** — what a newly created task starts with, see *Default values*
  below. Optional, and independent of *Required*.
- **Required** — see *What required means* below.

Fields are shown in the order of the list; drag a row to change it.

## The five types

| Type | Use it for | Stored as |
| --- | --- | --- |
| Text | short free text, up to 2000 characters | text |
| Number | scores, amounts, counts | a number, whole or decimal |
| Date | deadlines, review dates | a date |
| Select | a closed list you maintain | one of the field's options |
| Yes/No | a plain flag | a checkbox |

Pick *Select* over *Text* whenever the set of answers is known — it is the only
type where the tracker can guarantee the values stay consistent.

A *Yes/No* field is unset until someone ticks it. Unset and "no" are different
states in storage, but a plain checkbox shows both as unticked.

## Default values

A default is the value a task starts with. It is applied when the task is
created and nobody said otherwise — in the form it is simply prefilled, so you
can see it and overwrite it before saving; for an API or MCP client it fills the
keys the request left out. Sending the key with an explicit `null` still means
empty: "unspecified" and "empty" are different requests.

A default changes nothing on tasks that already exist. That is what *Required*'s
fill offer is for.

Two things follow from this:

- A required field that has a default can never block creation — the default
  always satisfies it.
- A *Select* field can only get a default once its options are saved, because the
  default points at one of them. Deleting that option clears the default.

## What required means

A required field **cannot be skipped when a task is created**, and its value
**cannot be removed** afterwards. Tasks that already existed when you turned the
requirement on may stay empty — the rule applies from that moment forward, and it
never blocks editing an older task. Such a task shows the field with a note saying
it became required after the task was created.

This is deliberate. Forcing a value onto hundreds of existing tasks only produces
`none` and `n/a`, and a made-up answer is worse than a blank one: a blank can be
found and filled later, a fake answer reads like the truth forever.

When you tick *Required* and tasks without a value exist, the form tells you how
many and offers to fill them all with one value. That offer is optional — saving
without it leaves them blank. Filling **never overwrites** a value someone already
entered.

Two consequences worth knowing:

- An agent or API client that creates a task must send every required field that
  has no default, or creation fails with an error naming the missing keys.
- An option of a required *Select* field cannot be removed by dropping its values;
  you have to move them to another option.

## The key

The key is how the API, the MCP tools and agents refer to the field. You do not
write it: it is derived from the name while you type. *Termín dodania* becomes
`termin_dodania`.

The rule is plain ASCII — accents are stripped, letters lowercased, and anything
else becomes an underscore. Two reasons: the key is a JSON key compared byte for
byte, and the same accented letter has two Unicode spellings, only one of which
would ever match; and a key gets typed by hand into API calls often enough that
it should be easy to type.

The *Key* field fills itself as you type the name and stays locked. *Edit* next to
it unlocks it if you want to choose your own; from then on it stops following the
name.

Two names can boil down to the same key — *Termín dodania* and *Termin dodania*
both give `termin_dodania`. The form says which field already holds that key and
refuses to save until you change the name or the key. An archived field still
counts: its values live on under its key.

A name written entirely outside the Latin alphabet leaves nothing to derive from,
so the form asks you for a key.

## Why the key cannot change

The key is the field's identity for everything outside the UI: REST clients,
MCP tools, and agents that were told to fill the field in. Renaming it would
silently break those callers, so the tracker refuses. The **name** is free to
change at any time — that is the label people read.

The **type** is immutable for the same reason, plus a second one: the values
already stored would no longer fit. If you need another type, create a new
field.

## Archiving vs deleting

**Archiving** stops new writes but keeps every value. The field disappears from
tasks that never had a value, and stays visible — read-only — on tasks that do.
This is what you want when a field falls out of use but its history still
matters.

**Deleting** removes the definition and every value with it, permanently. The
tracker tells you how many tasks are affected and asks for confirmation before
it does.

## Removing an option of a select field

An option that no task uses is removed without a word. An option that tasks
*do* use cannot simply vanish — that would leave those tasks with a value
pointing at nothing. The tracker asks you to choose:

- **move the values to another option** — the tasks keep a value, just a
  different one, or
- **delete the values** — the affected tasks end up with the field unset.

There is no third path. The database itself refuses to delete an option that is
still referenced, so a value can never quietly turn into a blank.

You can add an option and remove an in-use one in the same edit, and move the
values onto the option you have just typed. The tracker saves the addition first
so the new option is a real target by the time you pick it.

## Fields and agents

An agent sees the values of a task along with the task itself, so a field is a
way to hand it context that does not fit in the description.
