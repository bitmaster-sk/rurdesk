---
title: Agent Gateway
description: Deploy and configure the Goose agent gateway — local-first, or bring your own key.
---

# Agent Gateway

The gateway is a **stateless executor**. It receives `stage_execute` webhook
events from the API, drives an LLM coding agent against a per-run **git
worktree**, pushes a branch, and reports back. The **API** opens the PR/MR (via
the project's git integration) — the gateway only pushes. The API owns
scheduling and crash recovery; the gateway is a thin per-stage runner.

Each gateway runs **one stage at a time**. Further tasks for the same agent wait
in the tracker's queue. To work on more tasks in parallel, add another agent
with its own gateway.

You only need a gateway if you want the **AI Agent workflow** (tasks that agents
implement end-to-end). The in-API AI features (quality, kickstart, split) do
**not** use it.

## The Goose agent

The gateway drives [Goose](https://github.com/block/goose) — an open-source
coding agent. It is **provider-agnostic**: each entry of the `MODELS` list
names the provider it runs on. The same gateway image serves all four connection
options below, and one gateway can offer models from several of them at once;
you only change environment variables.

> **Local-first.** Point the gateway at a local **Ollama** and the whole agent
> workflow runs on your own hardware — no cloud, no API keys, nothing leaves the
> machine. The cloud options (Ollama Cloud, Anthropic, Gemini) are **bring-your-
> own-key** (BYOK): you set a provider key and billing is per-token against it.
> Either way there is no silent fallback — bad or missing credentials fail the
> run rather than degrading.

## Models

`MODELS` lists the models this gateway allows, as `provider/model` entries
separated by commas. The provider is the part before the first `/`; everything
after it is the model id, so `ollama/qwen3-coder:30b` keeps its tag.

```bash
MODELS=openai/kimi-k2.7-code,ollama/qwen3-coder:30b
MODEL_DEFAULT=openai/kimi-k2.7-code   # optional; the first entry otherwise
```

At startup the gateway reports this list to the tracker, which offers exactly
these models when a task is assigned to the agent (see
[Choosing the model](./agents.md#choosing-the-model)). Every `stage_execute`
names the model to run, and the gateway **refuses a model outside the list**: the
stage fails with `model_not_allowed` and the agent never starts. A tampered or
stale choice can therefore never run a model the gateway's operator did not
allow, such as an expensive one. When the tracker names no model, the gateway
runs `MODEL_DEFAULT`.

The provider keys stay in the gateway's environment, so the tracker never sees
them. Changing `MODELS` takes effect on the next gateway restart, which also
reports the new list. A task assigned to a model that is no longer listed runs
on the default instead, and its timeline says so.

The gateway refuses to start when `MODELS` is empty, an entry has no provider,
or `MODEL_DEFAULT` is not in the list.

## Connection options

Each `MODELS` entry starts with its provider. Set the variables of every
provider you list.

### Ollama (local) — zero cloud keys

Run Ollama on your own host, pull a model, and the agent never touches a cloud
service. No API key at all.

```bash
MODELS=ollama/qwen3-coder:30b                    # must be pulled on the host
OLLAMA_HOST=http://host.docker.internal:11434   # or http://gpu-box:11434
```

Pull the model first: `ollama pull qwen3-coder:30b`. This is the recommended
setup if you want everything self-contained — tickets, code, credentials, **and**
the model all stay on your infrastructure.

**Pick the model to fit the box.** `qwen3-coder:30b` is a mixture-of-experts
model — ~30B total but only a few billion active per token, so it answers fast
while staying far sharper than a dense 7B. It is the recommended starting point;
scale up if the machine allows.

| Machine                                   | Suggested model    | Rough footprint |
| ----------------------------------------- | ------------------ | --------------- |
| 32 GB RAM / Mac with 32 GB unified memory | `qwen3-coder:30b`  | ~20 GB          |
| Mac Studio / workstation, 96–128 GB       | `gpt-oss:120b`     | ~65 GB          |
| Mac Studio 512 GB, big GPU box            | `qwen3-coder:480b` | ~270 GB         |

Footprints are the default 4-bit quantisations and are approximate — check
`ollama list` after pulling. Leave headroom above the model size for context.

> The gateway is a **harder** workload than the in-API AI features: the agent
> runs a long tool-calling loop (read files, edit, run commands) and a model that
> drops or malforms tool calls burns its whole turn budget without
> finishing. Anything below the 30B row is unlikely to complete an
> implementation stage — prefer stepping up a row, or use a cloud provider.

### Ollama Cloud — hosted models, your key

[Ollama Cloud](https://ollama.com) hosts larger models than most local boxes can
run. Goose's native `ollama` provider **cannot authenticate** against it (it
sends no auth header and the host returns `401`), so Ollama Cloud is driven
through Goose's **OpenAI-compatible** provider instead:

```bash
MODELS=openai/kimi-k2.7-code
OPENAI_HOST=https://ollama.com
OPENAI_BASE_PATH=v1/chat/completions
OPENAI_API_KEY=<your-ollama-cloud-key>
```

> `kimi-k2.7-code` is the recommended model here — it handles the agent's
> tool-calling loop reliably. Model availability depends on your Ollama Cloud
> plan; other options include `qwen3-coder:480b`, `gpt-oss:120b`, and
> `gpt-oss:20b`. A `403 … requires a subscription` at run time means the chosen
> model is not on your plan.

### Anthropic — Claude models, your key

```bash
MODELS=anthropic/claude-sonnet-4-6
ANTHROPIC_API_KEY=sk-ant-...
```

Billing is per-token against your Anthropic API key (Console pay-per-token, not a
Pro/Max subscription).

### Gemini — Google models, your key

Goose reaches Gemini through its **`google`** provider:

```bash
MODELS=google/gemini-2.0-flash
GOOGLE_API_KEY=...
```

Billing is per-token against your Google AI API key.

## Required environment variables

The gateway refuses to start without all four:

| Var                        | Purpose                                                                                                                                                                                                                                                                                |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TRACKER_URL`              | Base URL of the tracker, e.g. `http://rurdesk:1000`. The fixed paths are appended in code: `/mcp/sse` for the agent's MCP client (non-implementation stages get the restricted `/mcp/plan/sse` subset automatically) and `/api/private` for REST calls (status, heartbeats, recovery). |
| `GATEWAY_TO_TRACKER_TOKEN` | The agent's API token, sent as Bearer on every tracker request and embedded in the agent's MCP config. **Issued from the tracker admin UI** (agent credentials).                                                                                                                       |
| `TRACKER_TO_GATEWAY_TOKEN` | Hex-encoded 32-byte HMAC token shared with the tracker. Verifies the `X-Tracker-Signature` on incoming `POST /event`. **Shown once when you register the agent's gateway** (see [Agents](./agents.md)).                                                                                |
| `MODELS`                   | The models this gateway allows, as `provider/model` entries separated by commas. See [Models](#models).                                                                                                                                                                                |

## Repositories

The tracker sends the repository with the work for the agent: the clone URL,
the base branch and the access token of the project's
[git integration](../using-the-tracker/git-integration.md). One gateway therefore
works for every project, whatever its git host, and nothing in its environment
changes when you add a project. Writing
[release notes](../using-the-tracker/releases.md#letting-an-agent-write-the-release-notes)
needs no repository.

- **Clone.** The gateway clones a repository the first time a run needs it, into
  `<WORKSPACE_BASE>/<host>/<repository path>`, e.g.
  `/worktrees/github.com/your-org/your-repo`. Every run gets its own worktree in
  that clone, branched from the freshly fetched base branch.
- **Base branch.** The tracker picks it when the run starts: the **Base branch**
  of the git integration, or the repository's default branch when it is empty.
  An empty repository gets `main`, and the gateway pushes an initial commit to it.
  Every stage of the run and its pull request use that same branch.
- **Token.** The token is never written to disk. The clone's git config holds a
  credential helper that reads the token from the environment of the git
  process, and the agent gets only the token of the run it works on.
- **Masking.** The gateway replaces the token with `***` in everything it sends
  to the tracker itself: the live thinking stream and error details. Text the
  agent posts through its own MCP tools does not pass through the gateway and is
  not masked.

The token needs to both open pull requests and push branches. See
[token scopes](../using-the-tracker/git-integration.md#token-scopes).

## Securing the tracker → gateway link

The webhook that starts a stage carries the git token of the run encrypted with
AES-GCM. The key is derived from `TRACKER_TO_GATEWAY_TOKEN`, which only the
tracker and that gateway know, so the token cannot be read on the way even over
plain HTTP. The HMAC signature proves the event came from the tracker.

The rest of the event is not encrypted: the task, the approved design and plan,
the comments and the wiki pages the agent gets. When that must not be readable
on the network, run the link over HTTPS or over a network nobody else can read
(the same compose network, a private VPN).

The gateway itself serves plain HTTP. For HTTPS put a reverse proxy in front of
it, for example Caddy, which also gets the certificate:

```yaml
services:
    gateway-proxy:
        image: caddy:2
        restart: unless-stopped
        command: caddy reverse-proxy --from gateway.example.com --to gateway-goose:9090
        ports:
            - "443:443"
        networks:
            rurdesk_network:
```

Then register the gateway in the tracker as `https://gateway.example.com/event`.
If the certificate comes from an internal CA, point the tracker at it with
`SSL_CERT_FILE=/path/to/ca.pem` (or `SSL_CERT_DIR`) in the tracker's environment.

## Optional environment variables

<!-- stack-table -->

| Var                         | Default          | Notes                                                                                                                                                                                          |
| --------------------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MODEL_DEFAULT`             | first of `MODELS` | The model a stage runs when the tracker names none. Must be listed in `MODELS`.                                                                                                                |
| `MAX_TURNS_LIMIT`           | _no limit_       | Highest turn limit a stage may run with. The gateway reports it to the tracker, which caps every limit it sends at this value; a stage that still asks for more fails with `max_turns_over_limit` and the agent never starts. The limits themselves are set in the tracker, see [Turn limits](./agents.md#turn-limits). |
| `LISTEN_PORT`               | `9090`           | Webhook receiver (`POST /event`) and healthcheck (`GET /health`).                                                                                                                              |
| `MAX_OUTPUT_LINE_MB`        | `4`              | Longest single line of agent output the gateway reads, in MB. A longer line is skipped and logged as a warning, and the stage keeps running.                                                   |
| `LOG_LEVEL`                 | `info`           | `debug` \| `info` \| `warn` \| `error` (JSON output).                                                                                                                                          |
| `WORKSPACE_BASE`            | `/worktrees`     | Root for the clones and their per-run worktrees; must match the workspace volume mount.                                                                                                        |

## Authentication

Goose needs **no interactive auth and no credentials volume** — it is
bring-your-own-key. List the models in `MODELS` and set the matching variables from
[Connection options](#connection-options) in the gateway's environment and the
key authenticates every run. For local Ollama there is no key at all.

There is nothing to seed once: the gateway rewrites the tracker MCP config
before every run (the endpoint is stage-scoped, so it differs per stage), and the
provider credentials are read straight from the environment.

## docker-compose snippet

Add this to the compose file from [Installation](../getting-started/installation.md)
— or run it as its own compose file on a different host, see the note below.

```yaml
services:
    gateway-goose:
        image: ghcr.io/bitmaster-sk/rurdesk/gateway-goose:latest
        restart: unless-stopped
        environment:
            TRACKER_URL: http://rurdesk:1000
            GATEWAY_TO_TRACKER_TOKEN: "" # CHANGE ME (Admin → agent user → Agent credentials)
            TRACKER_TO_GATEWAY_TOKEN: "" # CHANGE ME (… → Register gateway)
            LISTEN_PORT: "9090"
            LOG_LEVEL: info
            # --- models: list the ones this gateway allows, see "Models" ---
            MODELS: ollama/qwen3-coder:30b
            # MODELS: ollama/qwen3-coder:30b,openai/kimi-k2.7-code,anthropic/claude-sonnet-4-6
            # MAX_TURNS_LIMIT: "1000"
            # --- provider credentials: set those of every provider in MODELS ---
            # Ollama local (zero cloud keys):
            OLLAMA_HOST: http://host.docker.internal:11434
            # Ollama Cloud:
            # OPENAI_HOST: https://ollama.com
            # OPENAI_BASE_PATH: v1/chat/completions
            # OPENAI_API_KEY: ""
            # Anthropic:
            # ANTHROPIC_API_KEY: ""
            # Gemini:
            # GOOGLE_API_KEY: ""
        volumes:
            - gateway-goose-workspace:/worktrees
            - gateway-goose-state:/var/lib/gateway
        depends_on:
            - rurdesk
        networks:
            rurdesk_network:

volumes:
    gateway-goose-workspace:
    gateway-goose-state:
```

> No `ports:` mapping is needed when the gateway shares the tracker's compose
> network — the tracker posts events to `http://gateway-goose:9090/event`
> internally. Publish `9090` only if the gateway runs **separately** from the
> tracker (own compose file, own host, or next to a GPU box for local Ollama);
> then `TRACKER_URL` must point at the tracker's reachable URL and the gateway
> record in the app at the gateway's reachable `/event` URL.

> Never commit real tokens — fill the `CHANGE ME` values from the admin UI or an
> `.env` file kept out of version control.

## Wiring it to the tracker

The tracker side needs the agent's **gateway record** pointing at this container,
with the **matching `TRACKER_TO_GATEWAY_TOKEN` and `GATEWAY_TO_TRACKER_TOKEN`**.
The webhook target is the gateway's `/event` endpoint, e.g.
`http://gateway-goose:9090/event`.

Register it from the agent's credentials dialog in the admin UI — see
[Agents](./agents.md). The `TRACKER_TO_GATEWAY_TOKEN` is shown **once** at
creation time; copy it into the gateway's env (regenerate it if lost).

## Live thinking

The Goose gateway send the agent's thinking and tool calls to the tracker
while a stage runs, so the task's activity feed can show them live (see
[Watching the agent think](./agents.md#watching-the-agent-think)). It batches
them about once a second, and the whole channel is best-effort: if the tracker
rejects the batches — an older tracker, for instance — the gateway logs one
warning, stops relaying for that stage, and the run continues untouched.

## Healthcheck & verifying a deploy

```text
GET /health → {"ok":true}
```

1. `docker compose up -d gateway-goose`
2. `docker compose logs -f gateway-goose` — expect `gateway starting` /
   `gateway listening`. An early `gateway recovery report failed` is fine if the
   API hasn't fully come up yet.
3. Trigger a `stage_execute` event (assign a task to the agent — see
   [Agents](./agents.md)). Logs should show `starting goose` followed by
   structured `goose stream` records (the claude adapter logs `agent event`
   instead).
4. An authentication error means the provider credential is missing or wrong —
   confirm the provider of the model matches the variables you set (`OLLAMA_HOST` for
   local Ollama; `OPENAI_*` for Ollama Cloud; `ANTHROPIC_API_KEY` / `GOOGLE_API_KEY`
   for Anthropic / Gemini).
