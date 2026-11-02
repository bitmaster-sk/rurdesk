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

### Release notes runs need no repository

A `release_notes` run has a single `release_notes` stage after pickup. It
belongs to a release, not to a task, so its events carry no `idIssue`. The
tracker sends no repository for it, so the gateway runs the agent in an empty
scratch directory that it deletes afterwards. The prompt has its own header
without the task sections. It lists the tag, the closed tasks of the release and,
when the draft releases open tasks too, those under their own heading, and tells
the agent the list is complete. A release without tasks makes the agent end the
stage with an error instead of writing. The agent
writes the notes and hands them in as the message of `complete_stage`, without
a `message_kind`. The tracker puts that message into the release draft and posts
it nowhere else. The stage uses the restricted `/mcp/plan` endpoint like
every other non-implementation stage.

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
volume. The provider of the run's model (the part of its `MODELS` entry before
the first `/`) is passed as `GOOSE_PROVIDER` and `--provider`, and the matching
credential goes through to the child:

| Provider         | Credential                                                                   |
| ---------------- | ---------------------------------------------------------------------------- |
| `anthropic`      | `ANTHROPIC_API_KEY`                                                          |
| `google`         | `GOOGLE_API_KEY`                                                             |
| `ollama`         | `OLLAMA_HOST` (keyless; the model must be pulled on that host)               |
| `openai`         | `OPENAI_API_KEY` + `OPENAI_HOST` / `OPENAI_BASE_PATH` for OpenAI-compatible endpoints |

The shipped examples (`gateway-agent-007`, `gateway-agent-003`) use `openai`
pointed at Ollama Cloud. Note the
`openai` variables are not modelled in `GooseConfig` — they work because the
adapter passes the whole parent environment to the child (`childEnv`), so any
provider variable Goose itself understands gets through.

Goose supports more providers natively; only these are presented/tested today.
There is **no creds volume and no interactive auth** — the key is the billing
path, and a bad/missing key fails the run rather than degrading.

Pinned CLI version: **v1.56.0** (see [`Dockerfile`](goose/Dockerfile); `ARG GOOSE_VERSION`
plus one `ARG GOOSE_SHA256_*` per architecture). The image downloads the release
tarball directly and checks its sha256; the upstream install script is not used
because it falls back to the latest release when the pinned download fails.

The adapter also sets these goose variables for every run (`childEnv`). The
two with a default can be overridden in the gateway's environment:

| Variable                          | Value        | Why                                                                                                              |
| --------------------------------- | ------------ | ---------------------------------------------------------------------------------------------------------------- |
| `GOOSE_DEFAULT_EXTENSION_TIMEOUT` | default 1800 | goose kills a `shell` command after this many seconds (goose's own default 300 is too short for test suites)     |
| `GOOSE_DISABLE_SESSION_NAMING`    | true         | skips an extra model call goose makes to title the session                                                       |
| `GOOSE_STOP_HOOK_BLOCK_CAP`       | default 3    | how many times the Stop hook may send the agent back before goose ends the run anyway (goose's own default is 8) |
| `RURDESK_STAGE_MARKER`            | path         | per-task file the Stop hook uses to see that `complete_stage` was accepted                                       |

### How the tracker MCP is wired

Goose CLI flags can't carry an `Authorization` header on a remote extension, so
the tracker MCP server lives in `~/.config/goose/config.yaml`. It is a
**`streamable_http`** extension (goose ≥ v1.30 dropped SSE). The adapter
**writes this config per run** (`writeGooseConfig`) with the **literal**
stage-scoped URL + bearer token — goose does _not_ substitute `${ENV}` in an
extension `uri` (an unsubstituted value fails with a reqwest "builder error"),
and the URL differs by stage anyway: `/mcp/plan/http` for plan stages, `/mcp/http` for
implementation (converted from the `/sse` base by `toStreamableHTTP`). The config path is shared, which is safe because the
gateway runs one stage at a time. The API serves both transports: SSE (`/mcp/sse`) for the
claude-code adapter and streamable HTTP (`/mcp/http`) for goose.

> **Output mode:** the adapter runs goose with `--output-format stream-json`, so
> stdout is a per-token NDJSON event stream. A [`streamAggregator`](goose/stream.go)
> scans it live and **coalesces** the firehose into readable grouped log lines
> (`Msg("goose stream")`): the agent's _thinking_ at `debug`, the visible _answer_
> and _tool calls_ at `info` — so the operator can watch a run instead of seeing
> only a final blob. It also accumulates the token total (from the terminal
> `complete` event), the tool-call count, and the provider error. Goose reports
> a provider failure as an `error` content block with a `kind`
> (`authentication`, `creditsExhausted`, `other`, …) followed by a top-level
> `{"type":"error"}` event and exit 1; exhausted credit (HTTP 402) comes as a
> `systemNotification` instead, with exit 0. The adapter maps these to
> `provider_auth_failed`, `provider_credit_exhausted`, `provider_rate_limited`
> or `provider_error`, matching the text for `kind: other`. Run correctness
> still rides on the **exit code** + the agent's own MCP `complete_stage`, not
> on parsing stdout. Set `LOG_LEVEL=debug` to see thinking.
>
> **Missing-submission guard:** submission rides entirely on the agent calling
> `complete_stage` — the gateway never posts the output itself. A weaker model
> sometimes writes its result as a plain final message and exits 0 without
> submitting, which would hang the run in that stage forever. Two layers catch it:
>
> 1. The image ships a goose plugin, [`rurdesk-guard`](goose/plugins/rurdesk-guard)
>    in `/root/.agents/plugins`. Its `PostToolUse` hook creates
>    `RURDESK_STAGE_MARKER` once the tracker accepted `complete_stage`, and its
>    `Stop` hook blocks the end of the turn while the marker is missing, telling
>    the agent to call `complete_stage`. A call the tracker rejected (`isError`)
>    does not count. After `GOOSE_STOP_HOOK_BLOCK_CAP` blocks goose ends the run.
> 2. The aggregator counts `complete_stage` only when its tool response came
>    back without an error; if goose exits 0 (no provider error) without one, the
>    adapter fails the stage with `error_reason=stage_not_submitted` so the
>    tracker fails the run and the user gets Continue/Restart instead of a silent hang.
>
> **Repository plugins are ignored:** goose enables every plugin it finds in
> `<worktree>/.agents/plugins`, and their hooks run shell commands without the
> model deciding anything. `writeGooseConfig` lists each of them as
> `enabled: false` under `plugins:` in `config.yaml`, which a repository cannot
> override with its own `.config/goose/settings.json`.

### Required environment variables

These come from the tracker API and the models you allow; the gateway refuses to
start without all four. The other adapters share this exact
contract.

| Var                        | Purpose                                                                                                                                                                                                                                                                                                                          |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TRACKER_URL`              | Base URL of the tracker, e.g. `http://rurdesk.proxy`. The fixed endpoint paths are appended in code: `/mcp/sse` for the agent's MCP client (implement stage; every other stage gets the restricted `/mcp/plan/sse` subset automatically) and `/api/private` for REST calls (status reports, heartbeats, recovery report). |
| `GATEWAY_TO_TRACKER_TOKEN` | The agent's API token. Sent as Bearer on every tracker request and embedded in the agent's per-run MCP config so its MCP client authenticates the same way. Issued in the tracker admin UI (agent credentials).                                                                                                                      |
| `TRACKER_TO_GATEWAY_TOKEN` | **Hex-encoded** 32-byte HMAC token shared with the tracker. Used to verify the `X-Tracker-Signature` header on incoming `POST /event` calls. Mismatch → 401.                                                                                                                                                                     |
| `MODELS`                   | Comma-separated models this gateway allows. goose: `provider/model`, split on the first `/` (`ollama/qwen3:8b`). claude-code: bare model names (`sonnet,opus`), always on the subscription. Reported to the tracker in `ReportRecovered`; a `stage_execute` naming another model fails with `model_not_allowed` before the agent starts. |

> **Repositories come from the tracker.** Every `stage_execute` carries `repo`
> (`cloneUrl`, `host`, `repoPath`, `baseBranch`, `token`) from the project's git
> integration, so one gateway serves every project. `EnsureRepo` clones it on
> first use into `<WORKSPACE_BASE>/<host>/<repoPath>`. The clone's config holds a
> credential helper that reads `RURDESK_GIT_TOKEN` from the git process env, so
> the token never reaches disk; the agent process gets the token of its run in
> the same variable. The `TrackerClient` replaces the tokens of running stages
> with `***` in every body it posts. A `stage_execute` without `repo` fails with
> `tracker_missing_repo`.

### Optional environment variables

| Var                                    | Default          | Notes                                                                                                                                |
| -------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `LISTEN_PORT`                          | `9090`           | HTTP port for the webhook receiver (`POST /event`) and healthcheck (`GET /health`). Match the `ports:` mapping in your compose file. |
| `MAX_OUTPUT_LINE_MB`                   | `4`              | Longest line of agent output read, in MB. A longer line is skipped and logged; the stage keeps running.                              |
| `LOG_LEVEL`                            | `info`           | `debug` \| `info` \| `warn` \| `error`. JSON via `zerolog`.                                                                          |
| `WORKSPACE_BASE`                       | `/worktrees`     | Root for the clones and their per-run worktrees. Must match the `gateway-workspace` volume mount.                                    |
| `MODEL_DEFAULT`                        | first of `MODELS` | Model run when `stage_execute` names none. Must be in `MODELS`, or the gateway refuses to start.                                     |
| `MAX_TURNS_LIMIT`                      | no limit         | Ceiling on the `maxTurns` the tracker sends. Reported in `ReportRecovered` so the tracker caps at it; a higher value still fails with `max_turns_over_limit`. Without `maxTurns` the gateway uses 250 / 500 (implementation), capped by this limit. |
| `ANTHROPIC_API_KEY` / `GOOGLE_API_KEY` | unset            | BYOK credential for the `anthropic` / `google` provider.                                                                             |
| `OLLAMA_HOST`                          | unset            | For the `ollama` provider (keyless).                                                                                                 |
| `GOOSE_DEFAULT_EXTENSION_TIMEOUT`      | `1800`           | Seconds one goose `shell` command may run before goose kills it. |
| `GOOSE_STOP_HOOK_BLOCK_CAP`            | `3`              | How many times the `rurdesk-guard` Stop hook may send the agent back to call `complete_stage`. |

### docker-compose snippet

The dev compose ships one per agent: `gateway-agent-007` and `gateway-agent-003`.
They run the same image and differ only in their env file. One of them, verbatim:

```yaml
gateway-agent-007:
    # Built locally by ./script/build-gateway.sh — this tag is not on a registry.
    image: rurdesk-gateway-goose:latest
    container_name: gateway_agent_007
    env_file:
        - ./docker/gateway-agent-007/.env # copy from .env.example
    volumes:
        - gateway-agent-007-workspace:/worktrees
        - gateway-agent-007-state:/var/lib/gateway
        # NO creds volume — auth is API-key BYOK via env
    depends_on:
        - rurdesk.api
        - rurdesk.proxy
    networks:
        rurdesk_network:

volumes:
    gateway-agent-007-workspace:
    gateway-agent-007-state:
```

The required env (provider key, `MODELS` and the three tracker vars) lives in that
folder's `.env` — see
[`.env.example`](../docker/gateway-agent-007/.env.example). The tracker
also needs the webhook target configured
(`http://gateway_agent_007:9090/event`) and the matching
`TRACKER_TO_GATEWAY_TOKEN` and `GATEWAY_TO_TRACKER_TOKEN`.

### Upgrading from a gateway with its own repository settings

`REPO_URL`, `GIT_ACCESS_TOKEN` and `REPO_BRANCH_BASE` are ignored; `IgnoredRepoEnv`
logs a warning while they are set. On start `RemoveLegacyClones` deletes clones of
the old `<WORKSPACE_BASE>/<repo>` layout, whose `.git/config` held the token in the
remote URL. See [Upgrading to 1.6.0](../site/content/getting-started/upgrading.md#upgrading-to-160)
for the steps.

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
4. A provider/auth error means the wrong or missing key for the model's provider
   (or, for ollama, the model isn't pulled on `OLLAMA_HOST`).
