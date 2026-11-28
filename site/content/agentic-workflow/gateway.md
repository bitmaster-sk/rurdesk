---
title: Agent Gateway
description: Deploy and configure the agent gateway — Goose (local-first or bring your own key) or Claude Code (Claude Console account).
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

This page first covers what both agents share, then everything specific to
[Goose](#goose) and to [Claude Code](#claude-code).

## Choosing the agent

There are two gateway images. Each drives a different coding agent; the
tracker, the stages and the webhook protocol are the same for both. Pick one per
agent — different agents can use different images.

| Image                                          | Agent                       | Models and billing                                                           |
| ---------------------------------------------- | --------------------------- | ---------------------------------------------------------------------------- |
| `ghcr.io/bitmaster-sk/rurdesk/gateway-goose`  | [Goose](#goose)             | Local Ollama (no key), or your own key for Ollama Cloud / Anthropic / Gemini |
| `ghcr.io/bitmaster-sk/rurdesk/gateway-claude` | [Claude Code](#claude-code) | Claude models on a Claude Console API key, billed per token                  |

Pick **Goose** to run everything on your own hardware or to mix providers.
Pick **Claude Code** for Claude models driven by Anthropic's own coding agent.
Claude subscriptions (Pro, Max) cannot be used, see [Claude Code](#claude-code).

## Required environment variables

The gateway refuses to start without all four:

| Var                        | Purpose                                                                                                                                                                                                                                                                                |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TRACKER_URL`              | Base URL of the tracker, e.g. `http://rurdesk:1000`. The fixed paths are appended in code: `/mcp/http` for the agent's MCP client (non-implementation stages get the restricted `/mcp/plan/http` subset automatically), `/mcp/chat/http` for chat stages, and `/api/private` for REST calls (status, heartbeats, recovery). The agent reaches the tracker at this URL, so it must work from inside the gateway container. |
| `GATEWAY_TO_TRACKER_TOKEN` | The agent's API token, sent as Bearer on every tracker request and embedded in the agent's MCP config. **Issued from the tracker admin UI** (agent credentials).                                                                                                                       |
| `TRACKER_TO_GATEWAY_TOKEN` | Hex-encoded 32-byte HMAC token shared with the tracker. Verifies the `X-Tracker-Signature` on incoming `POST /event`. **Shown once when you register the agent's gateway** (see [Agents](./agents.md)).                                                                                |
| `MODELS`                   | The models this gateway allows, separated by commas. See [Models](#models).                                                                                                                                                                                                            |

The agent also needs its credentials: provider keys for
[Goose](#connection-options), a Console API key for
[Claude Code](#claude-code-api-key).

## Models

`MODELS` lists the models this gateway allows, separated by commas, and
`MODEL_DEFAULT` picks the one a stage runs when the tracker names none (the
first entry otherwise). The format of an entry depends on the agent: see
[Goose models](#goose-models) and [Claude Code models](#claude-code-models).

At startup the gateway reports this list to the tracker, which offers exactly
these models when a task is assigned to the agent (see
[Choosing the model](./agents.md#choosing-the-model)). Every `stage_execute`
names the model to run, and the gateway **refuses a model outside the list**: the
stage fails with `model_not_allowed` and the agent never starts. A tampered or
stale choice can therefore never run a model the gateway's operator did not
allow, such as an expensive one.

The credentials stay in the gateway's environment, so the tracker never sees
them. Changing `MODELS` takes effect on the next gateway restart, which also
reports the new list. A task assigned to a model that is no longer listed runs
on the default instead, and its timeline says so.

The gateway refuses to start when `MODELS` is empty, an entry does not fit the
agent's format, or `MODEL_DEFAULT` is not in the list.

### Thinking effort

The tracker sends the thinking effort of a stage (`low`, `medium`, `high` or
`max`) in `stage_execute`, see [Thinking effort](./agents.md#thinking-effort).
The gateway hands it to the agent; when the tracker sends none, the gateway sets
nothing and the model uses its own default. Any other value fails the stage with
`effort_not_allowed` before the agent starts.

An agent chat never sends an effort, so a chat turn always runs on the default.
For Claude Code that default is `medium`. To run stages higher, set the effort
in the agent or in the settings.

At startup the gateway also tells the tracker which of its `MODELS` think, so
the tracker offers an effort only for those. How each agent finds out is in
[Goose models](#goose-models) and [Claude Code models](#claude-code-models).

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
- **Worktree lifetime.** A run keeps its worktree when a stage fails or is
  cancelled, so **Continue**, or assigning an agent to the task again, resumes the
  unfinished work, a cancelled revision included: uncommitted changes and
  commits that were never pushed, or a rebase the agent stopped in the middle of.
  **Restart** removes the old run's worktree as soon as the gateway is not running
  another stage. Otherwise the gateway removes a worktree once nothing has used it for
  `WORKTREE_RETENTION_DAYS`. It then deletes the local branch too: right away when
  all its commits are on the git host, after `UNPUSHED_BRANCH_RETENTION_DAYS`
  when some never were.
- **Unusable worktree.** If a run's worktree has no branch checked out, because a
  newer run of the issue took over its pull request branch, or is no longer a git
  worktree, the stage fails with `worktree_unusable`. When the worktree holds
  nothing that could be lost, the gateway replaces it on its own instead. To
  recover, remove the directory named in the error on the gateway host (this
  needs shell access to it), then press **Continue**.
- **Base branch.** The tracker picks it when the run starts: the **Base branch**
  of the git integration, or the repository's default branch when it is empty.
  An empty repository gets `main`, and the gateway pushes an initial commit to it.
  Every stage of the run and its pull request use that same branch.
- **Token.** The token is never written to disk. The clone's git config holds a
  credential helper that reads the token from the environment of the git
  process, and the agent gets only the token of the run it works on.
- **Agent environment.** `GATEWAY_TO_TRACKER_TOKEN`, `TRACKER_TO_GATEWAY_TOKEN`
  and `RURDESK_GIT_TOKEN` are removed from the agent's environment in every stage;
  an issue stage gets the git token of its own run explicitly. Credential helpers
  already in the clone's config are reset, so the gateway's helper is the only one.
- **Chat.** [Agent chat](./agent-chat.md) uses the same clone for a detached,
  read-only worktree, see [The chat stage](#the-chat-stage).
- **Masking.** The gateway replaces the token with `***` in everything it sends
  to the tracker itself: the live thinking stream and error details. Text the
  agent posts through its own MCP tools does not pass through the gateway and is
  not masked.
- **Long commands.** One shell command of the agent may run for up to 30
  minutes by default, enough for a full test suite, and is stopped after that.
  The variables that change it are in [Goose settings](#goose-settings) and
  [Claude Code settings](#claude-code-settings).
- **Only its worktree.** The agent may commit and push only in the run's
  worktree. The pre-push hook refuses a push from anywhere else, such as the
  clone around it. If the agent changed the clone anyway, the gateway discards
  those changes before the next run on that repository and logs a warning;
  commits stay on their branch.

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

These apply to both agents. The variables of one agent are in
[Goose settings](#goose-settings) and [Claude Code settings](#claude-code-settings).

<!-- stack-table -->

| Var                               | Default           | Notes                                                                                                                                                                                                                                                                                                                   |
| --------------------------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MODEL_DEFAULT`                   | first of `MODELS` | The model a stage runs when the tracker names none. Must be listed in `MODELS`.                                                                                                                                                                                                                                         |
| `MAX_TURNS_LIMIT`                 | _no limit_        | Highest turn limit a stage may run with. The gateway reports it to the tracker, which caps every limit it sends at this value; a stage that still asks for more fails with `max_turns_over_limit` and the agent never starts. The limits themselves are set in the tracker, see [Turn limits](./agents.md#turn-limits). |
| `LISTEN_PORT`                     | `9090`            | Webhook receiver (`POST /event`) and healthcheck (`GET /health`).                                                                                                                                                                                                                                                       |
| `MAX_OUTPUT_LINE_MB`              | `4`               | Longest single line of agent output the gateway reads, in MB. A longer line is skipped and logged as a warning, and the stage keeps running.                                                                                                                                                                            |
| `LOG_LEVEL`                       | `info`            | `debug` \| `info` \| `warn` \| `error` (JSON output).                                                                                                                                                                                                                                                                   |
| `WORKSPACE_BASE`                  | `/worktrees`      | Root for the clones and their per-run worktrees; must match the workspace volume mount.                                                                                                                                                                                                                                 |
| `WORKTREE_RETENTION_DAYS`         | `7`               | Days a run worktree may stay unused before the gateway removes it. A failed or cancelled run keeps its worktree for Continue until then. Must be a whole number of at least 1. |
| `UNPUSHED_BRANCH_RETENTION_DAYS`  | `30`              | Days a local branch with commits that never reached the git host outlives its worktree. Branches whose commits are all on the host go together with the worktree. Must be a whole number of at least 1. |

## Deploying with Docker Compose

Each agent has its own compose snippet:
[Goose](#goose-compose-snippet) and [Claude Code](#claude-code-compose-snippet).
Add it to the compose file from
[Installation](../getting-started/installation.md) — or run it as its own
compose file on a different host, see the first note below. Both need a volume
for the worktrees (`/worktrees`) and one for runtime state (`/var/lib/gateway`).

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

The gateway sends the agent's thinking and tool calls to the tracker while a
stage runs, so the task's activity feed (or an [agent chat](./agent-chat.md))
can show them live (see
[Watching the agent think](./agents.md#watching-the-agent-think)). It batches
them about once a second, and the whole channel is best-effort: if the tracker
rejects the batches — an older tracker, for instance — the gateway logs one
warning, stops relaying for that stage, and the run continues untouched.

Both agents stream their thinking. Claude Code has a setting for it, see
[Claude Code settings](#claude-code-settings).

## The chat stage

An [agent chat](./agent-chat.md) message arrives as a `stage_execute` for the
`chat` stage. It differs from an issue stage:

- **Endpoint and token.** The agent's MCP client uses `/mcp/chat/http` with a
  **run token** (`rdr_…`). The token is scoped to one
  conversation, can only read (a fixed GET allowlist, no writes, no code and no
  global lists such as `get_my_issues` or `list_track_records`) and **expires at
  the end of each turn**, whether it completes, fails or is restarted. Even a
  turn that never ends cleanly cannot keep it longer than **6 hours** (a hard
  cap set when the turn starts). A stage without a token fails with
  `tracker_missing_run_token`.
- **Agent tools.** Which tools the agent keeps in chat, and whether it leaves a
  transcript, depends on the agent: see [Goose in chat](#goose-in-chat) and
  [Claude Code in chat](#claude-code-in-chat).
- **Environment.** The agent process never sees `GATEWAY_TO_TRACKER_TOKEN`,
  `TRACKER_TO_GATEWAY_TOKEN` or `RURDESK_GIT_TOKEN`. This applies to every stage;
  issue stages get the git token explicitly for their own run, chat gets none.
  Credential helpers found in the clone's git config are reset to the gateway's own.
- **Worktree.** With a project, a **detached** worktree is reused from turn to
  turn without a fetch and reset to the head recorded after the previous turn
  (`reset --hard`, `clean`, with hooks and fsmonitor off for the gateway's git
  calls). When the default branch moved on, it is recreated at the current
  commit and the conversation gets a "code refreshed" note. The gateway reports
  `headCommit` in the stage's stats. Without a project, or for an empty
  origin, the agent gets a scratch directory and nothing is cloned or pushed.
- **Retention.** Chat worktrees are removed after `WORKTREE_RETENTION_DAYS`
  like others.
- **Scheduling.** The tracker runs a chat turn after the stage that is active
  and before queued issue runs, and re-queues it when the gateway restarts, up to 3
  attempts per message. Steady chat traffic on a shared agent therefore delays
  its queued issue runs.

## Healthcheck & verifying a deploy

```text
GET /health → {"ok":true}
```

1. `docker compose up -d gateway-goose` (or `gateway-claude`)
2. `docker compose logs -f gateway-goose` — expect `gateway starting` /
   `gateway listening`. An early `gateway recovery report failed` is fine if the
   API hasn't fully come up yet.
3. Trigger a `stage_execute` event (assign a task to the agent — see
   [Agents](./agents.md)). The log lines that show the agent at work are in
   [Checking Goose](#checking-goose) and
   [Checking Claude Code](#checking-claude-code).
4. An authentication error means the agent's credential is missing or wrong;
   what to check is in the same sections.

## Goose

The Goose gateway drives [Goose](https://github.com/aaif-goose/goose) — an open-source
coding agent. It is **provider-agnostic**: each entry of the `MODELS` list
names the provider it runs on. The same gateway image serves all four
[connection options](#connection-options), and one gateway can offer models
from several of them at once; you only change environment variables.

> **Local-first.** Point the gateway at a local **Ollama** and the whole agent
> workflow runs on your own hardware — no cloud, no API keys, nothing leaves the
> machine. The cloud options (Ollama Cloud, Anthropic, Gemini) are **bring-your-
> own-key** (BYOK): you set a provider key and billing is per-token against it.
> Either way there is no silent fallback — bad or missing credentials fail the
> run rather than degrading.

### Goose models

Each `MODELS` entry is `provider/model`: the provider is the part before the
first `/`; everything after it is the model id, so `ollama/qwen3-coder:30b`
keeps its tag. An entry without a provider stops the gateway at startup.

```bash
MODELS=openai/kimi-k2.7-code,ollama/qwen3-coder:30b
MODEL_DEFAULT=openai/kimi-k2.7-code   # optional; the first entry otherwise
```

Goose gets the [thinking effort](#thinking-effort) as `GOOSE_THINKING_EFFORT`
and maps it to what the provider understands. Goose maps it only for some
providers, such as OpenAI reasoning models and xAI. For other models behind the
`openai` provider, such as glm, kimi or mistral on Ollama Cloud, the effort has
no effect.

To report which models think, Goose asks Ollama (`OLLAMA_HOST`,
`http://localhost:11434` when unset) which of its `ollama/*` models have no
thinking, so the tracker does not offer an effort for them. Other providers are
left to the models.dev list the tracker ships. This is the only call besides the
model itself, and it stays on your own Ollama. It does not delay the gateway's
start. If Ollama is not up yet, the gateway keeps asking for a few minutes. A
model Ollama never answers for keeps what the tracker knew about it from the
previous start; a model it has never answered for is unknown, and the tracker
still offers the effort for it.

### Connection options

Each `MODELS` entry starts with its provider. Set the variables of every
provider you list.

#### Ollama (local) — zero cloud keys

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

#### Ollama Cloud — hosted models, your key

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

#### Anthropic — Claude models, your key

```bash
MODELS=anthropic/claude-sonnet-4-6
ANTHROPIC_API_KEY=sk-ant-...
```

Billing is per-token against your Anthropic API key (Console pay-per-token).

#### Gemini — Google models, your key

Goose reaches Gemini through its **`google`** provider:

```bash
MODELS=google/gemini-2.0-flash
GOOGLE_API_KEY=...
```

Billing is per-token against your Google AI API key.

### Goose authentication

Goose needs **no interactive auth and no credentials volume** — it is
bring-your-own-key. List the models in `MODELS` and set the matching variables
from [Connection options](#connection-options) in the gateway's environment and
the key authenticates every run. For local Ollama there is no key at all.

There is nothing to seed once: the gateway rewrites the tracker MCP config
before every run (the endpoint is stage-scoped, so it differs per stage), and the
provider credentials are read straight from the environment.

### Goose settings

| Var                               | Default | Notes                                                                                                                                       |
| --------------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `GOOSE_DEFAULT_EXTENSION_TIMEOUT` | `1800`  | Seconds one shell command of the agent may run before Goose stops it.                                                                       |
| `GOOSE_STOP_HOOK_BLOCK_CAP`       | `3`     | How many times an agent that tries to stop without submitting its result is sent back to submit it. Each reminder uses a turn.              |

### Goose in chat

In the [chat stage](#the-chat-stage) Goose keeps its tools, including the shell;
see [Known limitations](./agent-chat.md#known-limitations). It runs every stage
with `--no-session`, so it leaves no transcript.

### Goose compose snippet

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
            # --- models: list the ones this gateway allows, see "Goose models" ---
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

### Checking Goose

- A stage logs `starting goose` followed by structured `goose stream` records.
- An authentication error means a provider credential is missing or wrong.
  Check that the provider of the model matches the variables you set:
  `OLLAMA_HOST` for local Ollama, `OPENAI_*` for Ollama Cloud,
  `ANTHROPIC_API_KEY` / `GOOGLE_API_KEY` for Anthropic / Gemini.

## Claude Code

The Claude Code gateway drives Anthropic's
[Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI. It runs only
Claude models, on a **Claude Console account** billed per token. The runs count
against the rate limits of the Console organization; when one is hit, the stage
fails with `provider_rate_limited`.

> **Claude subscriptions are not supported.** Anthropic's terms do not allow a
> Free, Pro, Max or Ultra subscription to be used by a shared service on behalf
> of its users, so do not sign the gateway in with one. See
> [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance).

The image pins the CLI version, so an upstream release cannot change the
agent's behaviour under you.

### Claude Code models

Each `MODELS` entry is a model name as the CLI accepts it, without a provider:
an alias such as `sonnet`, `opus` or `haiku`, or a full model id such as
`claude-sonnet-5-5`.

An alias follows the CLI in the image, so a CLI bump can move it to a newer
model. With CLI 2.1.332, `sonnet` is `claude-sonnet-5-5` and `opus` is
`claude-opus-5-5`. A full model id pins the model. Agents, stages and chats
store the name from `MODELS`, so if you swap an alias for a full id, every saved
choice of the alias falls back to `MODEL_DEFAULT`.

```bash
MODELS=sonnet,opus,haiku
MODEL_DEFAULT=sonnet   # optional; the first entry otherwise
```

Claude Code gets the [thinking effort](#thinking-effort) as `--effort`, and the
gateway reports all its models as thinking.

### Claude Code API key

Create an API key for the gateway in the
[Claude Console](https://platform.claude.com) under **API Keys**. Choose how
long it stays valid and the workspace it belongs to, so the agent's usage and
limits are kept apart from the rest of the organization. A key that belongs to
no workspace fails every stage with
`400 This API key is not scoped to a workspace`. The Console organization pays
for every token the agent uses.

Put the key into the gateway's environment under this name:

```bash
CLAUDE_CODE_OAUTH_TOKEN=<api key>
```

Do not set `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN`: the gateway removes
them before the agent starts, so the key above is the only credential the
agent sees.

Without the variable, the CLI uses a login stored in the
[credentials volume](#claude-code-credentials-volume). To store one, open the
CLI in the gateway container once and run `/login`:

```bash
docker compose exec -it gateway-claude claude
```

Sign in with your Console account, not a subscription. With neither a key nor a
login, every stage fails with an authentication error. When the key expires,
the stages fail the same way until you set a new one.

> Treat the key and the volume like a password: whoever has them runs Claude
> at your Console organization's expense. Revoke a leaked key in the Console.

### Claude Code credentials volume

The Claude Code gateway needs a third volume, mounted at **`/root/.claude`**.
The CLI keeps its state there: the stored login, its settings and the
transcript of every stage. Without the volume a recreated container loses
them, and a gateway that signs in with `/login` has to sign in again. Chat
turns leave no transcript there, see [Claude Code in chat](#claude-code-in-chat).

### Claude Code settings

| Var                       | Default      | Notes                                                                                                   |
| ------------------------- | ------------ | ------------------------------------------------------------------------------------------------------- |
| `BASH_DEFAULT_TIMEOUT_MS` | `1800000`    | Milliseconds one shell command of the agent may run when the agent sets no timeout of its own.          |
| `BASH_MAX_TIMEOUT_MS`     | `1800000`    | Longest timeout, in milliseconds, the agent may set for one shell command.                              |
| `CLAUDE_THINKING_DISPLAY` | `summarized` | How the agent's thinking is streamed to [Live thinking](#live-thinking), see below.                     |

The agent runs with `--include-partial-messages` and
`--thinking-display summarized`, so it streams thinking fragments; the text is
the API's **summarised** thinking, not the raw chain of thought.
`CLAUDE_THINKING_DISPLAY` sets the display value: unset means `summarized`;
`summarized` or `omitted` is passed as is; **empty** omits the flag, for
example when a Claude Code version rejects it. Any other value stops the
gateway at startup with an error. These flags are re-tested on every bump of
the pinned CLI version.

### Claude Code in chat

In the [chat stage](#the-chat-stage) the agent is read-only:

- It runs with `--tools Read,Grep,Glob,WebSearch,WebFetch`, an allow-list of
  built-in tools, so no built-in that runs commands (`Bash`, `Monitor`, ...),
  edits files or starts subagents is offered;
  `--disallowedTools Bash,Write,Edit,NotebookEdit,Task` denies the
  main ones again. MCP tools are not built-ins and stay callable.
- The web tools are there for research: a page the agent fetches can carry
  instructions, and a fetched URL can carry data out.
- `Read`, `Grep` and `Glob` can read any file the gateway process can read, not
  only the worktree. See [Known limitations](./agent-chat.md#known-limitations).
- It runs with `--no-session-persistence`, so the CLI writes no session file
  under `~/.claude/projects`.

### Claude Code compose snippet

```yaml
services:
    gateway-claude:
        image: ghcr.io/bitmaster-sk/rurdesk/gateway-claude:latest
        restart: unless-stopped
        environment:
            TRACKER_URL: http://rurdesk:1000
            GATEWAY_TO_TRACKER_TOKEN: "" # CHANGE ME (Admin → agent user → Agent credentials)
            TRACKER_TO_GATEWAY_TOKEN: "" # CHANGE ME (… → Register gateway)
            LISTEN_PORT: "9090"
            LOG_LEVEL: info
            MODELS: sonnet,opus,haiku
            # MAX_TURNS_LIMIT: "1000"
            CLAUDE_CODE_OAUTH_TOKEN: "" # CHANGE ME (API key from the Claude Console)
        volumes:
            - gateway-claude-workspace:/worktrees
            - gateway-claude-state:/var/lib/gateway
            - gateway-claude-creds:/root/.claude
        depends_on:
            - rurdesk
        networks:
            rurdesk_network:

volumes:
    gateway-claude-workspace:
    gateway-claude-state:
    gateway-claude-creds:
```

To sign in with `/login` instead of the key, leave `CLAUDE_CODE_OAUTH_TOKEN`
out, see [Claude Code API key](#claude-code-api-key).

### Checking Claude Code

- A stage logs `starting claude code` followed by `agent event` records.
- An authentication error means the key is missing, expired or revoked. Check
  `CLAUDE_CODE_OAUTH_TOKEN`, or the stored login if you signed in with
  `/login`; an expired or revoked key needs a new one from the Console.
- `400 This API key is not scoped to a workspace` means the key belongs to no
  workspace; create one in a workspace.
