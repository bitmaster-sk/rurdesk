---
title: API keys
description: Create an API key that signs in as you, use it to connect an MCP client such as Claude Code, and revoke it when a machine is no longer yours.
---

# API keys

An **API key** is a long-lived credential that authenticates as you. A request carrying
one acts as you across your projects, with the same roles, so a key is as sensitive as
your password. The one thing it cannot do is reach the administration console — see
[Limits](#limits).

Other tools call the same thing a *personal access token*. It works the same way here.

Keys exist for clients that cannot sign in through a browser. The main one is an **MCP
client** such as Claude Code, which connects to the tracker's MCP server and works on
your tasks as you.

## Creating a key

1. Open **User settings**.
2. Scroll to **API keys**.
3. Enter a name that says where the key will live — `Laptop`, `CI`, `Claude Code`.
4. Optionally pick an expiry date. Leave it empty and the key never expires.
5. Click **Create key**.

The raw key is shown **once**, right after creation. Copy it then — the tracker stores
only a hash and cannot show it again. If you lose it, regenerate the key or create a
new one.

Create one key per client rather than sharing a single key everywhere. That way, losing
a laptop costs you one key instead of every integration you have.

## Connecting an MCP client

Point the client at the tracker's MCP endpoint and pass the key as a bearer token:

```
https://<your-instance>/mcp/http
Authorization: Bearer <key>
```

`/mcp/http` is the streamable HTTP transport; `/mcp/sse` serves the same tool set over
SSE. The plan-stage variants live under `/mcp/plan/http` and `/mcp/plan/sse` and expose
read-only tools plus `submit_plan` / `request_clarification`.

## Rotating and revoking

**Regenerate** replaces a key's value in place, keeping its name and its row. The old
value stops working immediately, so update the client before you rotate.

**Revoke** deletes the key. Any client still using it starts failing at once — a revoked
key is rejected on the very next request, not after a cache delay.

## Limits

- A key cannot be used to manage keys. Requests authenticated with a key are rejected on
  the key endpoints, so a leaked key cannot mint more; revoking it really ends the
  incident.
- A key never carries administrator rights, even when you are an administrator. Requests
  to the admin endpoints are refused. This is deliberate: with admin rights a leaked key
  could create a bot and mint that bot's key, which is the same self-perpetuation the
  rule above prevents. Use the web interface for admin work.
- A key cannot raise its own rate limit. Keys run at the standard per-user limit.
- The number of keys per user is capped. An administrator sets the cap under
  **Admin → Settings → API keys per user**; the default is 10. Raising it never touches
  existing keys, and lowering it only blocks new ones.
