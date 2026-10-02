# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2026-09-09

Watch an agent think while it works, answer its review comments without sending it back
to code, and track your time from anywhere in the app.

### Highlights

- **Live agent thinking** — the agent's reasoning and tool calls appear in the task's
  activity feed as they happen, not after the stage ends. The thinking of a running stage
  survives a reload, and an instance admin decides whether it is kept afterwards and how
  much of it.
- **Reply to a review comment** — an agent can answer a question about its work without
  pushing new code. It also reads the whole review thread and the whole task thread, so a
  later instruction now beats the plan you approved earlier.
- **Personal API keys** — every user can create, list, regenerate and revoke their own API
  keys in user settings, separate from the keys that belong to agents. An instance admin
  caps how many keys a user may hold.
- **Time tracking from anywhere** — start a timer from the task detail, quick actions or
  the command palette. Only one runs at a time, it shows in the page header with today's
  total, it syncs across browser tabs, and you can pause, resume and add a note when you
  submit it.
- **Agents are called agents** — the word "bot" is gone from the UI, the API, the gateway
  and the documentation. See [Breaking changes](#breaking-changes).

### Added

- Agent thinking stored, served over REST and pushed over WebSocket, with a live row in
  the activity feed, readable tool call details, a notice when events are missing, and
  `agentThinkingMaxKb` plus `isAgentThinkingPersisted` as instance settings.
- `review_reply` message kind and its rendering in the activity feed.
- Live pull request status in the task detail: the pill updates as CI and the merge state
  change, without a reload.
- Personal API keys: `GET`/`POST`/`DELETE` under `/user/api-key`, a section in user
  settings, an instance setting for the maximum per user, and documentation.
- Tracker pause, resume and submit-with-note in the API, a running-timer control in the
  header with a day chart, a dialog when you start a second timer, tracker entries in the
  command palette, and editable track records that refresh in place.
- Emoji picker in the message composer — a curated grid, no new dependencies. ASCII
  smileys still convert as you type.
- Shared form validators for "not blank" and "future date".
- End-to-end coverage for agent thinking (against a stub gateway, no LLM), the workflow
  event to state transitions, personal API keys and the timer lifecycle.

### Changed

- **The bot vocabulary is now the agent vocabulary** across database columns, JSON
  fields, gateway configuration and every user-facing string. See
  [Breaking changes](#breaking-changes).
- The implementation stage is marked "PR opened" only when the run really has a pull
  request, and output submitted without a branch is rejected until it does.
- An agent run branches from a freshly fetched origin base, detected from the remote when
  `REPO_BRANCH_BASE` is unset.
- The merge poll interval is documented and its default lives in one place.
- The task table shows a loading state and an empty state instead of an endless spinner.
- The MR link picker preselects the git integration when there is only one.
- Documentation covers agent thinking, reviewing an open pull request through task
  comments, and personal API keys.

### Fixed

- A markdown pipeline could corrupt code blocks while converting emoji; the conversion now
  happens in the composer as you type instead.
- The task table kept spinning forever when its first fetch failed.
- A disabled button passed its click through to the element behind it.
- The calendar followed the app language and no longer throws on an unsupported locale.
- Notifications loaded only after the top menu had rendered.
- The user settings content scrolls instead of overflowing the page.
- Task type badges render in the dropdown, and the type field matches the height of the
  fields around it.
- The task filter debounces only the title, not the whole form.
- Various type errors that CI never saw — spec files are now type-checked in CI, and
  `skipLibCheck` is off.

### Breaking changes

**"Bot" was renamed to "agent" throughout.**

JSON fields change: `isBot` → `isAgent`, `idUserBot` → `idUserAgent`, `idBotGateway` →
`idGateway`. The gateway's `bot` configuration field is now `agent`. Database columns and
tables are migrated for you (`agent.bot_gateway` becomes `agent.gateway`). Direct API
callers and any gateway configuration file need updating; the UI is updated.

### Database

Five migrations, applied automatically on startup:

| Migration                                                   | Purpose                                              |
| ----------------------------------------------------------- | ---------------------------------------------------- |
| `20260829120000_user_api_key.sql`                           | personal API keys, split from agent keys             |
| `20260831120000_agent_task_thinking_and_result_message.sql` | stored agent thinking and stage result messages      |
| `20260904120000_message_kind_review_reply.sql`              | the `review_reply` message kind                      |
| `20260907120000_rename_bot_to_agent.sql`                    | renames the bot columns, tables and indexes to agent |
| `20260908120000_tracker_pause_and_track_note.sql`           | timer pause state and a note on a track record       |

Rolling back to 1.2.0 requires `goose down`; removing the image is not enough.

### Upgrading

```bash
docker compose pull
docker compose up -d
```

Migrations run on startup. Before upgrading, check whether you call the API with `isBot`,
`idUserBot` or `idBotGateway`, and update your gateway configuration — see
[Breaking changes](#breaking-changes).

[1.3.0]: https://github.com/bitmaster-sk/rurdesk/compare/v1.2.0...v1.3.0

## [1.2.0] - 2026-08-29

Task types, agent skills and real CI status from every git host — plus a pass over the
whole app so it can be driven from the keyboard.

### Highlights

- **Task types** — Bug, Feature and Task out of the box, editable per project like states
  and severities. Filter by them in every view, set them in bulk, pick a default for new
  tasks, and find the tasks that have none.
- **Agent skills** — reusable instruction blocks you curate once and attach to a project
  and a workflow stage. The agent gets them in its prompt. Four skills ship with the app
  and stay up to date on upgrade unless you have edited them; you can also change which
  skills a run uses right up until its stage starts.
- **Real CI status on pull requests** — GitHub, GitLab and Gitea now show what the
  pipeline actually did, including canceled and skipped runs, and read approvals from the
  host. Previously these were placeholder values.
- **Link an existing pull request yourself** — no agent run needed. A manually linked PR
  gets the same status pill, diff, merge tracking and automatic state change as one the
  agent opened.

### Added

- Task type as a per-project field: manage the list in project settings, choose a default
  for new tasks, filter by type (including "no type set"), change it on many tasks at
  once, and use it from the agent tools.
- Skill catalog under admin, a per-project skill matrix in project settings, and the
  skills of a running task shown and editable on its run card.
- Skills and workload dock in the assignee dropdown, so you can see what each agent is
  already working on before you assign.
- Assign an agent directly from a task, choosing its skills as you do.
- Resizable split on the task detail: drag the divider, collapse either side, and the
  position is remembered. The task info fields reflow to fit the width you leave them.
- "Stay logged in" on the login form — the session lasts 30 days instead of a day.
- The whole app is now operable from the keyboard, with a visible focus ring that follows
  the brand colour: notification cards, team rows, sprint tabs, pins, activity filters,
  relation badges, diff file headers, participants and panel headers all respond to Enter
  and Space. The command palette announces itself properly to screen readers, and the
  multiselect selects everything with Ctrl+A.
- The browser tab shows the open task, and quick actions can open a task in a new tab.
- Documentation for agent skills, task types, automatic state changes, manual PR linking
  and what each CI status label means.

### Changed

- **"Agent phase → state map" is now "Workflow event → state map"** in the UI and in the
  API. See [Breaking changes](#breaking-changes-1).
- **Editing a task through the API no longer clears the fields you did not send.** See
  [Breaking changes](#breaking-changes-1).
- Agents get a far higher turn limit (250 for planning, 500 for implementation, up from
  50/100), and running out of turns now says so and offers Continue instead of looking
  like the agent simply never submitted its work.
- Descriptions and comments render better: line breaks behave the way they do on GitHub,
  and headings, inline code, code blocks, quotes and tables match the rest of the app.
- When an agent retries a stage, the task shows the result of the newest attempt.

### Fixed

- Agents and project members got "access denied" when reading project context.
- The command palette found no tasks when opened before the project had finished loading.
- On the gantt chart the date tooltip could be hidden behind task bars while dragging,
  dependency arrows stole keyboard focus and wore the browser's own focus ring, arrow
  colours drifted apart during highlighting, and clicking elsewhere did not deselect.
- Enter did nothing on a select's clear button, and row actions could not be reached from
  the keyboard.
- The focus ring was cut off inside panels and cards.
- Copying a freshly revealed bot token failed over plain HTTP; the dialog is now wider and
  the token can be selected by hand.
- Text with accented characters could be cut mid-character in notifications and agent
  error messages.
- Filtering out closed tasks failed on the server.
- A failed instance-settings save could leave some values written and others not.
- Table group headings were styled wrong, and a few labels sat in the wrong section.

### Security

- The app no longer sends your session token anywhere but its own backend. It used to
  attach the `Authorization` header to any request, including ones to third-party URLs,
  and sent an empty header when you were logged out.

### Breaking changes

**The agent phase → state map was renamed.**

`GET`/`PUT` on `/project/:idProject/agent-phase-state-map` are now
`/project/:idProject/workflow-event-state-map`, and each mapping's `phase` field is now
`event`. Existing configuration is migrated for you. Only direct API callers are
affected; the UI is updated.

**Editing a task through the API is now a partial update.**

A `PATCH` used to be applied as a full replacement: any field left out of the body was
written back as empty, so a small edit could silently wipe the title, description or
assignee. Only the fields you send now change, and sending an explicit `null` clears one.
If you relied on omission to clear a field, send it explicitly.

**`mrID` is now `idMr`.** Update any direct API calls that pass the old name.

### Database

Three migrations, applied automatically on startup:

| Migration                                     | Purpose                                                                                    |
| --------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `20260821120000_issue_type.sql`               | task types, with Bug/Feature/Task seeded for every project                                 |
| `20260823120000_workflow_event_state_map.sql` | renames the phase→state map to the workflow event→state map and records a PR's merge state |
| `20260824120000_agent_skill.sql`              | skill catalog and its per-project, per-stage mapping                                       |

Rolling back to 1.1.0 requires `goose down`; removing the image is not enough.

### Upgrading

```bash
docker compose pull
docker compose up -d
```

Migrations run on startup. Before upgrading, check whether you call the phase→state map
endpoints or edit tasks over the API directly — see [Breaking changes](#breaking-changes-1).

[1.2.0]: https://github.com/bitmaster-sk/rurdesk/compare/v1.1.0...v1.2.0

## [1.1.0] - 2026-08-18

Saved views, cycle analytics, and relative date filters — plus TypeScript `strict`
and ESLint across the client.

### Highlights

- **Saved views** — store a filter, sort and layout as a named view and reopen it in one
  click across all four issue views (table, kanban, calendar, gantt). Shareable per
  project or kept private, each with its own URL, and reachable from the command palette.
- **Cycle analytics** — a health strip above the kanban board showing progress, pace,
  forecast and verdict, with a Charts toggle for burndown and velocity. A daily
  background snapshot keeps the charts cheap to render.
- **Relative date filters** — filter issues by rolling windows such as "last 7 days" on
  both creation and last-update time. The window is resolved server-side against a
  reference time held in the cursor, so results don't shift while you page through them.

### Added

- Saved view CRUD API with config validation, plus store, converter and command palette
  entries; saved views apply, save and deep-link across all four issue views.
- Sprint health strip above the kanban board: progress, pace, forecast and verdict, with
  a points/tasks toggle that follows the UTC day rollover.
- Burndown and velocity charts, served from a daily sprint snapshot recorded by a new
  generic interval scheduler.
- `sprints.velocity_limit` app setting so an instance admin can choose how many recent
  cycles the velocity average covers.
- Relative `created_within` and `updated_within` filters, exposed on the MCP `list_issues`
  tool as well as the UI.
- `ui-date-range-select` component combining rolling presets with a fixed range;
  `uiDatepicker` can now render inline in any mode, range included.
- Duration parser accepting unit combinations such as `1d8h6m`.
- Delete dialog for states and severities that asks what should happen to the issues
  still using them.
- Dependency licence checking in CI; third-party notices are generated at release time
  and shipped inside the container images.
- End-to-end coverage for the issue lifecycle through a reload, the command palette in a
  real project, the sprint health strip and the charts band. Failed runs now keep their
  trace, video and screenshot.

### Changed

- **Deleting a state or severity that is still in use now returns `409`** and requires an
  explicit `migrateTo` parameter. See [Breaking changes](#breaking-changes-2).
- An empty team list serializes as `[]` instead of `null`.
- TypeScript `strict` is enabled across the client, including `strictTemplates`, and CI
  now type-checks the frontend — previously a `strict` violation merged green and only
  surfaced in the nightly end-to-end run a day later.
- ESLint is enforced in CI (ESLint 10, `typescript-eslint` 8, `angular-eslint` 21) with
  42 rules. Bringing the code in line moved constructor injection to `inject()`, renamed
  component outputs that collided with DOM event names, made translation lookups return
  `string` instead of `any`, and removed redundant type assertions throughout.
- Fonts are served locally as woff2 instead of from `fonts.googleapis.com`, so neither
  the landing page nor the documentation calls a third party.
- Global list hotkeys no longer fire while an overlay is open.
- The gantt chart frame no longer includes the filter panel above it.
- Table sort headers can be driven by a consumer, not only by clicks.

### Fixed

- Multi-value ID filters were silently dropped because the array parameters were not sent
  in a form the server understood.
- Closing a window with the ✕ corrupted Angular's view state.
- Dropping an issue onto the calendar when it has no scheduled date now reverts instead
  of leaving an inconsistent state.
- An instance admin could not see projects the ACL already granted them.
- Resizing the calendar threw when the FullCalendar API was not ready yet.
- `ui-select` and `ui-multiselect` tolerate `null` entries in their option lists.
- The filter panel now hydrates from the live filter when it mounts late.
- CDK accessibility styles are loaded, so live announcements stay visually hidden.

### Security

- Concurrent first-user registration returned `500` and could bootstrap an instance with
  two instance admins. The existence check and the insert are now a single atomic step
  guarded by a transaction-scoped advisory lock; the losing request gets `403`.

### Breaking changes

**Deleting a state or severity that is still in use requires explicit intent.**

`DELETE` on a state or severity now returns `409` with code `STATE_IN_USE` or
`SEVERITY_IN_USE` when the item is still referenced — by issues, as a project default, or
in an agent phase mapping. The caller must say what should happen:

- `migrateTo=<id>` — move the affected issues to another state or severity
- `migrateTo=null` — clear the value on the affected issues

Deleting an unused state or severity is unchanged and needs no parameter.

In 1.0.0 this was a plain `DELETE` with no check: it either failed on a foreign key or
left dangling references behind. If you call the API directly, update those calls before
upgrading. The web UI handles this for you with a dialog.

**`idsTeams` is never `null`.** The project member response returns an empty array
instead. Clients that treated `null` and `[]` differently need to be adjusted.

### Database

Three additive migrations, applied automatically on startup:

| Migration                                | Purpose                                         |
| ---------------------------------------- | ----------------------------------------------- |
| `20260803120000_saved_view.sql`          | saved views table                               |
| `20260811120000_issue_backlog_index.sql` | partial index for the backlog aggregate         |
| `20260812120000_sprint_snapshot.sql`     | daily cycle snapshots for burndown and velocity |

Rolling back to 1.0.0 requires `goose down`; removing the image is not enough.

### Upgrading

```bash
docker compose pull
docker compose up -d
```

Migrations run on startup. Before upgrading, check whether you call `DELETE` on states or
severities directly through the API — see [Breaking changes](#breaking-changes-2).

[1.1.0]: https://github.com/bitmaster-sk/rurdesk/compare/v1.0.0...v1.1.0
