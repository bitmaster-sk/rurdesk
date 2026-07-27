---
title: User Management
description: Accounts, the global admin role, bots, bot credentials, teams, and project roles.
---

# User Management

Two permission layers exist side by side:

| Layer | Scope | Values |
| --- | --- | --- |
| **Global admin** | Whole instance | `admin` flag on a user |
| **Project role** | One project | `viewer` · `member` · `owner` |

## Accounts & first run

Public registration at **`/register`** is a **one-time bootstrap**: the first
ever account created on the instance **automatically becomes the global
admin**, and the endpoint closes permanently afterwards. Every further account
— human or bot — is created by an admin from the UI. Opening `/register` on an
already-bootstrapped instance shows a "Registration is closed" notice — ask
your admin for an account instead.

Users log in at **`/login`**. Avatars are generated from the user's initials on
a colour they pick in their profile — nothing is fetched from a third party.

![Login screen](../../site/assets/img/login.png)

## The admin area

Admins get an **Administration** page (the **Users** entry in the top menu,
`/admin/users`) with two panels: a **Users** list of every account on the
instance — humans and bots — and a **Teams** panel (see [Teams](#teams)).

![Admin user list](../../site/assets/img/admin-users.png)

From the Users panel an admin can:

- **Create human users** — name, e-mail, password, optional **admin** flag,
  and optionally assign them straight to a project with a role.
- **Create bots** — see below.
- **Promote / demote admins** — the **last admin can never be demoted or
  deleted** (guarded server-side), and **bots can never be admins**.
- **Delete users** — refused while the user has agent activity (runs,
  provenance records), so history stays attributable.

> Changing a user's admin flag or deleting a user **invalidates their active
> sessions** immediately — no stale privileges survive.

![Create user dialog](../../site/assets/img/create-user-dialog.png)

## Bots

A **bot** is a non-human account that an [agent gateway](./gateway.md) acts as.
Bots are created from the same dialog with the **Bot** toggle:

- The e-mail is synthesized (`bot-<name>@bots.local`, domain configurable via
  `BOT_EMAIL_DOMAIN`) and the password is random and discarded — **a bot can
  never log in interactively**.
- A **default API token is minted on creation** and shown **once** — copy it
  right away; it becomes the gateway's `GATEWAY_TO_TRACKER_TOKEN`.
- The **Tracker → Gateway token** (`TRACKER_TO_GATEWAY_TOKEN`, the secret the
  tracker presents when it calls the gateway) is likewise displayed **only at
  the moment it is created or regenerated** — copy it then; it can never be read
  back, only replaced.
- Bots can be project members or viewers, but **never project owners** and
  never admins.

### Bot credentials

API tokens are **bot-only**. The admin user list has a credentials dialog per
bot (Gateway → Tracker tokens and the Tracker → Gateway token):

- **List** keys (name, created date — never the raw key)
- **Create** additional named keys — raw value shown once
- **Revoke** keys immediately

Both directions are **shown once, at creation time only** — the Gateway →
Tracker key when it is created or regenerated, the Tracker → Gateway token when
the gateway is registered or its token regenerated. Neither can be read back
later; a lost value must be regenerated, which invalidates the old one.

Rotation:

- **Gateway → Tracker** (no downtime): create the new token → switch the
  gateway's `GATEWAY_TO_TRACKER_TOKEN` → revoke the old token.
- **Tracker → Gateway**: regenerating replaces the token immediately, so update
  the gateway's `TRACKER_TO_GATEWAY_TOKEN` and restart it right after —
  webhook calls fail signature verification in between.

![Bot credentials dialog](../../site/assets/img/bot-keys.png)

## Teams

A **team** is an instance-wide group of users, managed **only by admins** in the
**Teams** panel of the admin page. There are no invitations or self-service team
creation — an admin owns the whole lifecycle:

- **Create / edit / delete** a team (name + color).
- **Add members** by dragging users from the Users panel onto a team (or via the
  team's add-member control); **remove members** from the team's roster.

Teams exist to grant several users the same project role in one step: an owner
attaches a team to a project (see below) and every member inherits that role.
**Deleting a team removes the project access it granted** to its members.

![Admin teams panel](../../site/assets/img/admin-teams.png)

## Project roles

Membership is managed per project in **Project settings → Members** by a
project **owner**. The owner adds **individual users** or attaches an
**existing team** (created by an admin), each with a role. A user's effective
role is the **highest** of their direct assignment and any team they belong to.
Owners manage a project's membership — they do **not** create or edit the teams
themselves; that stays in the [admin Teams panel](#teams).

| Capability | viewer | member | owner |
| --- | :-: | :-: | :-: |
| See the project, tasks, views | ✔ | ✔ | ✔ |
| Create / edit tasks, log time, relations | | ✔ | ✔ |
| Read git integrations, MR diffs | | ✔ | ✔ |
| Manage project members & team access | | | ✔ |
| Manage git integrations | | | ✔ |
| Manage bot gateways & agent settings | | | ✔ |

> **Time entries.** A time entry can be edited or removed only by its author, or
> by a project **owner** (who may correct or clean up any member's entries). A
> **viewer cannot edit even their own entries** — logging or changing time
> requires project write access.

![Project members](../../site/assets/img/project-members.png)

## Next steps

- [Agents](./agents.md) — wire a bot to a gateway and run the agent workflow
- [Git integration](./git-integration.md) — connect repositories to a project
