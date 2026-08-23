# Gateway

The gateway is a stateless executor that picks up `stage_execute` webhook events
from the issue-tracker API, drives an LLM agent against a per-run git worktree,
and reports the result back. The API owns scheduling and crash recovery; the
gateway is a thin per-stage runner.

Two adapters exist today:

| Adapter         | Status            | Auth                                                                    | Docs                          |
| --------------- | ----------------- | ----------------------------------------------------------------------- | ----------------------------- |
| **goose**       | Reference adapter | **API key (BYOK)** — Anthropic / Google / Ollama                        | [below](#goose-adapter)       |
| **claude-code** | Available         | Claude subscription OAuth (interactive, one-off) + optional setup-token | not yet documented            |

All adapters share the same `common/` runtime (webhook receiver, orchestrator,
worktree management, prompt rendering, MCP wiring) and differ only in how they
drive their respective agent. Pick one per gateway instance via the `--adapter`
flag (the Docker images set it for you).

### Prior outputs are injected into the prompt

Each stage runs a fresh agent process, so to avoid re-fetching prior work the
gateway renders the **approved design** and **approved implementation plan**
(shipped by the API in the `stage_execute` `contextBundle`) directly into the
per-stage prompt, above the instructions. The implementation-plan stage sees the
design; the implementation stage sees both. The agent is told to work from the
injected copy instead of pulling it back from the tracker, which cuts tool
round-trips and tokens on every stage — independently of (and more cheaply than)
any session-resume scheme.

---

## goose adapter

This is the reference adapter; the shared runtime contract (required env, MCP
wiring, healthcheck) is documented here and the other adapters refer back to it.

### How auth works

The adapter shells out to Block's [Goose](https://github.com/block/goose) agent
(`goose run`, a pinned Rust binary — [`adapter.go`](goose/adapter.go)). Goose
authenticates with a **per-token API key (BYOK)**, not a subscription OAuth
volume. `GOOSE_PROVIDER` selects the provider and the adapter passes the matching
credential through to the child:

| `GOOSE_PROVIDER` | Credential                                                                   |
| ---------------- | ---------------------------------------------------------------------------- |
| `anthropic`      | `ANTHROPIC_API_KEY`                                                          |
| `google`         | `GOOGLE_API_KEY`                                                             |
| `ollama`         | `OLLAMA_HOST` (keyless; the model must be pulled on that host)               |
| `openai`         | `OPENAI_API_KEY` + `OPENAI_HOST` / `OPENAI_BASE_PATH` for OpenAI-compatible endpoints |

The shipped examples cover two of these: `gateway-goose-qwen-local` runs
`ollama` against a local host, while `gateway-goose-qwen-cloud` and
`gateway-goose-kimi-cloud` use `openai` pointed at Ollama Cloud. Note the
`openai` variables are not modelled in `GooseConfig` — they work because the
adapter passes the whole parent environment to the child (`childEnv`), so any
provider variable Goose itself understands gets through.

Goose supports more providers natively; only these are presented/tested today.
There is **no creds volume and no interactive auth** — the key is the billing
path, and a bad/missing key fails the run rather than degrading.

Pinned CLI version: **v1.38.0** (see [`Dockerfile`](goose/Dockerfile); `ARG GOOSE_VERSION`).

### How the tracker MCP is wired

Goose CLI flags can't carry an `Authorization` header on a remote extension, so
the tracker MCP server lives in `~/.config/goose/config.yaml`. It is a
**`streamable_http`** extension (goose ≥ v1.30 dropped SSE). The adapter
**writes this config per run** (`writeGooseConfig`) with the **literal**
stage-scoped URL + bearer token — goose does _not_ substitute `${ENV}` in an
extension `uri` (an unsubstituted value fails with a reqwest "builder error"),
and the URL differs by stage anyway: `/mcp/plan/http` for plan stages,
`/mcp/http` for implementation (converted from the `/sse` base by
`toStreamableHTTP`). The config path is shared, so this assumes
`MAX_CONCURRENT=1`. The API serves both transports: SSE (`/mcp/sse`) for the
claude-code adapter and streamable HTTP (`/mcp/http`) for goose.

> **Output mode:** the adapter runs goose with `--output-format stream-json`, so
> stdout is a per-token NDJSON event stream. A [`streamAggregator`](goose/stream.go)
> scans it live and **coalesces** the firehose into readable grouped log lines
> (`Msg("goose stream")`): the agent's _thinking_ at `debug`, the visible _answer_
> and _tool calls_ at `info` — so the operator can watch a run instead of seeing
> only a final blob. It also accumulates the token total (from the terminal
> `complete` event), the tool-call count, and a provider-error classification
> (goose surfaces provider failures as exit-0 assistant text — block/goose#4612).
> Run correctness still rides on the **exit code** + the agent's own MCP
> `complete_stage`, not on parsing stdout. Set `LOG_LEVEL=debug` to see thinking.
>
> **Missing-submission guard:** submission rides entirely on the agent calling
> `complete_stage` — the gateway never posts the output itself. A weaker model
> sometimes writes its result as a plain final message and exits 0 without
> submitting, which would hang the run in that stage forever. The aggregator
> tracks whether `complete_stage` was called; if goose exits 0 (no provider
> error) but never called it, the adapter fails the stage with
> `error_reason=stage_not_submitted` so the tracker fails the run and the user
> gets Continue/Restart instead of a silent hang.

### Required environment variables

These come from the tracker API and your git provider; the gateway refuses to
start without all five. The other adapters share this exact contract.

| Var                        | Purpose                                                                                                                                                                                                                                                                                                                          |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TRACKER_URL`              | Base URL of the tracker, e.g. `http://rurdesk.proxy`. The fixed endpoint paths are appended in code: `/mcp/sse` for the agent's MCP client (implement stage; non-implementation stages get the restricted `/mcp/plan/sse` subset automatically) and `/api/private` for REST calls (status reports, heartbeats, recovery report). |
| `GATEWAY_TO_TRACKER_TOKEN` | The bot's API token. Sent as Bearer on every tracker request and embedded in the agent's per-run MCP config so its MCP client authenticates the same way. Issued in the tracker admin UI (bot credentials).                                                                                                                      |
| `TRACKER_TO_GATEWAY_TOKEN` | **Hex-encoded** 32-byte HMAC token shared with the tracker. Used to verify the `X-Tracker-Signature` header on incoming `POST /event` calls. Mismatch → 401.                                                                                                                                                                     |
| `REPO_URL`                 | The single git remote this gateway works in, cloned into `/worktrees` at startup; per-run worktrees are checked out from that clone. One repo per gateway — see the note below.                                                                                                                                                  |
| `GIT_ACCESS_TOKEN`         | Personal access token / app token with contents read+write on `REPO_URL`. Injected into the remote URL so the per-run agent can push branches back. Only **push** is needed — the tracker (API) opens the PR/MR via the project's git-integration token, not this one.                                                          |

> **One repo per gateway.** A project in the tracker may hold several git
> integrations, and the tracker resolves which one a run used by matching the
> repo the gateway reports back (`ReportRunRepo`). Nothing today records which
> repo a given *issue* belongs to, so the gateway cannot choose per task — it
> always works in `REPO_URL`. To cover a second repo, run a second gateway with
> its own bot.

### Optional environment variables

| Var                                    | Default          | Notes                                                                                                                                |
| -------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `LISTEN_PORT`                          | `9090`           | HTTP port for the webhook receiver (`POST /event`) and healthcheck (`GET /health`). Match the `ports:` mapping in your compose file. |
| `MAX_CONCURRENT`                       | `1`              | Soft cap on in-flight runs per gateway instance. The shared MCP config path assumes `1` — keep it there.                             |
| `LOG_LEVEL`                            | `info`           | `debug` \| `info` \| `warn` \| `error`. JSON via `zerolog`.                                                                          |
| `WORKSPACE_BASE`                       | `/worktrees`     | Root for per-run git worktrees. Must match the `gateway-workspace` volume mount.                                                     |
| `REPO_BRANCH_BASE`                     | `main`           | Branch each per-run worktree starts from.                                                                                            |
| `GOOSE_PROVIDER`                       | `anthropic`      | `anthropic` \| `google` \| `ollama`.                                                                                                 |
| `GOOSE_MODEL`                          | provider default | Model id; unrecognised name fails on first call.                                                                                     |
| `ANTHROPIC_API_KEY` / `GOOGLE_API_KEY` | unset            | BYOK credential for the chosen provider.                                                                                             |
| `OLLAMA_HOST`                          | unset            | For `GOOSE_PROVIDER=ollama` (keyless).                                                                                               |
| `GOOSE_MAX_TURNS_PLAN`                 | `250`            | Hard `--max-turns` for non-implementation stages.                                                                                    |
| `GOOSE_MAX_TURNS_IMPLEMENT`            | `500`            | Hard `--max-turns` for implementation.                                                                                               |

### docker-compose snippet

The dev compose ships three of these, one per bot: `gateway-goose-qwen-local`,
`gateway-goose-qwen-cloud` and `gateway-goose-kimi-cloud`. They all run the same
image and differ only in their env file. One of them, verbatim:

```yaml
gateway-goose-qwen-local:
    # Built locally by ./script/build-gateway.sh — this tag is not on a registry.
    image: rurdesk-gateway-goose:latest
    container_name: gateway_goose_qwen_local
    env_file:
        - ./docker/gateway-goose-qwen-local/.env # copy from .env.example
    volumes:
        - gateway-goose-qwen-local-workspace:/worktrees
        - gateway-goose-qwen-local-state:/var/lib/gateway
        # NO creds volume — auth is API-key BYOK via env
    depends_on:
        - rurdesk.api
        - rurdesk.proxy
    networks:
        rurdesk_network:

volumes:
    gateway-goose-qwen-local-workspace:
    gateway-goose-qwen-local-state:
```

The required env (provider key + the five tracker/git vars) lives in that
folder's `.env` — see
[`.env.example`](../docker/gateway-goose-qwen-local/.env.example). The tracker
also needs the webhook target configured
(`http://gateway_goose_qwen_local:9090/event`) and the matching
`TRACKER_TO_GATEWAY_TOKEN` and `GATEWAY_TO_TRACKER_TOKEN`.

### Healthcheck

```
GET /health → {"ok":true}
```

The image has a built-in Docker `HEALTHCHECK` that curls this every 30s.

### Verifying a fresh deploy

1. `docker compose up -d gateway-goose`
2. `docker compose logs -f gateway-goose` — expect `gateway starting`.
   `gateway recovery report failed` is fine if the API hasn't fully come up yet.
3. Trigger a `stage_execute` event. Logs should show `starting goose` followed
   by structured `goose stream` records (the claude adapter logs `agent event`
   instead).
4. A provider/auth error means the wrong or missing key for `GOOSE_PROVIDER`
   (or, for ollama, the model isn't pulled on `OLLAMA_HOST`).
