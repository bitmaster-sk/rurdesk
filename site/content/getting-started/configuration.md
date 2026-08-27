---
title: Configuration
description: API environment variables — database, cache, AI provider, and more.
stackTables: true
---

# Configuration

The API is configured entirely through environment variables (read via Viper's
`AutomaticEnv`). In the Docker setup these live in **`docker/api/.env`**.

> **Never commit real secrets.** All values below are placeholders. Generate
> strong random values for keys and passwords.

## Database (PostgreSQL)

| Variable | Example | Purpose |
| --- | --- | --- |
| `DATABASE_HOST` | `rurdesk-db` | Postgres host (the compose network alias) |
| `DATABASE_PORT` | `5432` | Postgres port |
| `DATABASE_NAME` | `issue` | Database name |
| `DATABASE_USER` | `issue` | Database user |
| `DATABASE_PASSWORD` | `<strong-password>` | Database password |

These must match the credentials your Postgres service is configured with.

## Cache (Redis)

| Variable | Example | Purpose |
| --- | --- | --- |
| `CACHE_HOST` | `rurdesk-cache` | Redis host (compose alias) |
| `CACHE_PORT` | `6379` | Redis port |
| `CACHE_PASSWORD` | _(empty in dev)_ | Redis password, if set |
| `CACHE_DB` | `0` | Redis logical DB index |

## Application

| Variable | Example | Purpose |
| --- | --- | --- |
| `APPLICATION_HOST` | `0.0.0.0` | Bind host for the tracker |
| `APPLICATION_PORT` | `1000` | Single listen port — serves REST, WebSocket, MCP (`/mcp`) and, in production, the Angular SPA |
| `SERVE_STATIC` | `true` | Production only: serve the Angular build from the Go binary (set in the `rurdesk` image; leave unset in dev) |
| `STATIC_DIR` | `/app/public` | Directory of the built SPA when `SERVE_STATIC=true` |
| `ALLOWED_WS_ORIGINS` | _(empty)_ | Extra origin hosts (comma-separated) allowed to open the WebSocket. Empty by default — only **same-origin** handshakes are accepted. See the reverse-proxy note below |
| `ALLOWED_ORIGINS` | _(empty)_ | Comma-separated origins allowed to call the API from a browser. Empty means `*` — safe here because authentication is a bearer token, never a cookie, so a wildcard grants no ambient authority. Set it to have the browser enforce a narrower set. |
| `MCP_PUBLIC_BASE_URL` | _(empty)_ | Pins the origin advertised to MCP clients in the `/mcp` endpoint URL. Empty by default — derived from each request's `Host`. Set it only when your proxy cannot preserve `Host` |
| `GIT_INTEGRATION_ENCRYPTION_KEY` | `<32-byte-key>` | Encrypts stored git integration tokens at rest |

> **WebSocket origin & your reverse proxy.** Real-time updates use a WebSocket
> that is accepted only when the request's `Origin` matches its `Host` (a
> same-origin guard against Cross-Site WebSocket Hijacking). The browser
> authenticates this socket by sending its session token in the handshake's
> `Sec-WebSocket-Protocol` header, and the origin guard is kept as defense in
> depth. **Your TLS terminator / reverse proxy must forward the original
> `Host` header unchanged** (e.g. nginx `proxy_set_header Host $host;`) — if it
> rewrites `Host`, real-time updates stop working. When the SPA is served from a
> **different host** than the API (split-host deployment), list the SPA's host
> in `ALLOWED_WS_ORIGINS` (comma-separated), e.g. `app.example.com`.

> **`GIT_INTEGRATION_ENCRYPTION_KEY`** (32-byte, base64) encrypts stored git
> access tokens at rest with AES-256-GCM. Set it once and keep it stable — just
> swapping the value leaves the app unable to decrypt existing tokens.

**Rotating the key.** The production image ships an `admin` maintenance CLI for
this — a one-off command, not an API call. Run it **inside the running
container**: it re-encrypts every stored token from the old key to a new one in a
single transaction (a wrong old key rolls back and changes nothing), using the
container's own `DATABASE_*` env. Afterwards set the app's
`GIT_INTEGRATION_ENCRYPTION_KEY` to the new key and restart.

```bash
# 1. generate a new 32-byte key
openssl rand -base64 32

# 2. rotate inside the container — prompts (masked) for the current + new key
docker exec -it <container> admin rotate-git-key
#   Current encryption key (base64): ********
#   New encryption key (base64): ********
#   rotated N token(s)

# 3. set GIT_INTEGRATION_ENCRYPTION_KEY to the new key and restart the app
```

Non-interactively (e.g. from a script) pass both keys as env instead of typing
them at the prompt:

```bash
docker exec \
  -e GIT_INTEGRATION_ENCRYPTION_KEY="<current-key>" \
  -e NEW_ENCRYPTION_KEY="<new-key>" \
  <container> admin rotate-git-key
```

> **There is no auth signing secret to configure.** Sessions are opaque random
> tokens held in Redis and looked up per request — not signed JWTs — so there is
> no equivalent of a `JWT_SECRET` to set or rotate. Signing out or flushing the
> session cache invalidates tokens immediately. The only auth-adjacent variables
> are `ALLOWED_WS_ORIGINS` (above) and `GIT_INTEGRATION_ENCRYPTION_KEY`.

### Password hashing cost

Passwords and bot secrets are hashed with bcrypt at cost 10. `BCRYPT_COST`
overrides that, and values outside bcrypt's accepted range (4–31) are ignored in
favour of the default.

**Leave it unset.** It exists so the test suite can hash at cost 4 — bcrypt is a
tight CPU loop that the Go race detector slows roughly 12x, which turned the
integration suite into a seven-minute bcrypt benchmark. Lowering it on a real
deployment weakens every password hashed while it is in effect, and raising it
makes each login proportionally slower (each `+1` doubles the work).

## AI provider (Quality Check, Kickstarter, Task Split)

These features run **inside the API** and call the provider you select here.
Pick a cloud provider **or** run **Ollama locally** to avoid cloud API keys
entirely.

| Variable | Example | Purpose |
| --- | --- | --- |
| `AI_PROVIDER` | `anthropic` | One of `anthropic` (default), `openai`, `gemini`, `ollama` |
| `AI_API_KEY` | `<provider-key>` | API key for the chosen cloud provider (not needed for `ollama`) |
| `AI_MODEL` | `<model-id>` | Model the AI features use — no built-in default. Optional: leave unset and the tracker runs fine without AI; the AI features just return an "AI not configured" toast until you set it |
| `AI_HOST` | _(provider default)_ | Base URL — required for `ollama` and self-hosted/openai-compatible endpoints |
| `AI_QUALITY_MODEL` | _(falls back to `AI_MODEL`)_ | Override model used specifically for the quality checker |
| `AI_TIMEOUT_SECONDS` | `300` | Per-request timeout for a provider call. Raise for slow cloud or large local models |
| `PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH` | `10000` | Max length of the project description input (default 10000) |

### AI is optional

The AI features are **opt-in**, not required to run the tracker. There are no
built-in model defaults — you choose the model explicitly with `AI_MODEL`. If it
is unset (and `AI_QUALITY_MODEL` too), only the AI features (Quality Check,
Kickstarter, Task Split) are affected: each returns an **"AI is not configured"**
error and the client shows a toast. Everything else — projects, tasks, views,
sprints, git integration — works exactly the same.

| `AI_PROVIDER` | Needs `AI_API_KEY` | Needs `AI_HOST` |
| --- | --- | --- |
| `anthropic` (default) | yes | optional |
| `openai` | yes | optional (for OpenAI-compatible endpoints) |
| `gemini` | yes | no |
| `ollama` | **no** | **yes** (e.g. `http://host.docker.internal:11434`) |

### Zero-cloud-key setup (Ollama)

```env
AI_PROVIDER=ollama
AI_HOST=http://host.docker.internal:11434
AI_MODEL=qwen3-coder:30b
```

Run Ollama on the host and pull the model first (`ollama pull qwen3-coder:30b`).
With this, quality check, kickstart, and split work with no external API keys.

**Pick the model to fit the box.** These features ask the model to reason over a
task description and answer in a fixed structure, so a small general chat model
(7B and below) tends to ramble or break the format — give it something with real
capacity. `qwen3-coder:30b` is a mixture-of-experts model: ~30B total but only a
few billion active per token, so it answers fast while staying far sharper than a
dense 7B. It is the recommended starting point.

| Machine | Suggested `AI_MODEL` | Rough footprint |
| --- | --- | --- |
| 32 GB RAM / Mac with 32 GB unified memory | `qwen3-coder:30b` | ~20 GB |
| Mac Studio / workstation, 96–128 GB | `gpt-oss:120b` | ~65 GB |
| Mac Studio 512 GB, big GPU box | `qwen3-coder:480b` | ~270 GB |

Footprints are the default 4-bit quantisations and are approximate — check
`ollama list` after pulling. Leave headroom above the model size for context.
If a pull is too big for the machine, step down a row rather than swapping to a
small dense model.

> **Ollama Cloud (with an API key) uses `openai`, not `ollama`.** The `ollama`
> provider is native/local-only — it sends no `Authorization` header and calls
> `/api/chat`, so it cannot talk to `ollama.com`. For Ollama Cloud use its
> OpenAI-compatible endpoint instead:
> ```env
> AI_PROVIDER=openai
> AI_HOST=https://ollama.com      # base only — the provider appends /v1/chat/completions
> AI_API_KEY=<your-ollama-cloud-key>
> AI_MODEL=kimi-k2.7-code
> ```

> **Slow models and proxy timeouts.** Large cloud/local models can take minutes
> to answer. The API bounds each call with `AI_TIMEOUT_SECONDS` (default 300s),
> but any reverse proxy in front of it must allow at least as long. The dev nginx
> proxy sets `proxy_read_timeout`/`proxy_send_timeout` to 600s on `/api/` for this
> reason (nginx otherwise defaults to 60s). If you raise `AI_TIMEOUT_SECONDS`,
> raise the proxy timeout to match.

> The **agent gateway** is independent of this provider — it runs the Goose agent
> against its own `GOOSE_PROVIDER` (local Ollama, Ollama Cloud, Anthropic, or
> Gemini), not `AI_API_KEY`. See [Agent Gateway](./gateway.md).

## Minimal example `docker/api/.env`

```env
# Database
DATABASE_HOST=rurdesk-db
DATABASE_PORT=5432
DATABASE_NAME=rurdesk
DATABASE_USER=rurdesk
DATABASE_PASSWORD=rurdesk

# Cache
CACHE_HOST=rurdesk-cache
CACHE_PORT=6379
CACHE_PASSWORD=
CACHE_DB=0

# Application (single port: REST + WebSocket + MCP + SPA)
APPLICATION_HOST=0.0.0.0
APPLICATION_PORT=1000
GIT_INTEGRATION_ENCRYPTION_KEY=<generate-a-32-byte-key>

# AI features (in-API): quality check / kickstarter / split
AI_PROVIDER=anthropic
AI_API_KEY=<your-anthropic-key>
AI_MODEL=<model-id>          # no default; unset = AI features off (toast), rest works
# AI_QUALITY_MODEL=          # optional — falls back to AI_MODEL
PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH=10000
```


## Applying changes

After editing `docker/api/.env`:

```bash
docker compose up -d --force-recreate rurdesk.api
```
