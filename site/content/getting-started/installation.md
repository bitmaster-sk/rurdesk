---
title: Installation
description: Run the stack from published GHCR images with Docker Compose.
---

# Installation

The platform ships as **published container images on GHCR** — no source
checkout, no build step. One `docker-compose.yml` brings up the whole stack.

## Prerequisites

- **Docker** 24+ and the **Docker Compose** plugin
- ~2 GB free RAM for the stack
- Port **80** free on the host (only the tracker is published)
- TLS is expected to be terminated by an external load balancer/ingress; the
  tracker itself is HTTP-only. That proxy **must forward the original `Host`
  header unchanged** — the WebSocket (real-time updates) is same-origin guarded
  and rejects handshakes whose `Host` was rewritten. See
  [Configuration → WebSocket origin](./configuration.md#application)

## 1. Create the compose file

Save the following as `docker-compose.yml` in an empty directory and edit the
values marked `CHANGE ME`. This is the deployment stack; the repository's
`README.md` covers the separate development compose instead.

In production the entire web stack is a **single `rurdesk` image**: the Go
binary serves the REST API, WebSocket, the MCP endpoints and the Angular SPA on
one port. (The separate client/API/proxy containers exist only in the
development setup.)

```yaml
services:
    rurdesk:
        image: ghcr.io/bitmaster-sk/rurdesk/rurdesk:latest
        container_name: rurdesk
        restart: unless-stopped
        ports:
            - "80:1000"
        environment:
            DATABASE_HOST: rurdesk_db
            DATABASE_NAME: rurdesk
            DATABASE_USER: rurdesk
            DATABASE_PASSWORD: rurdesk          # CHANGE ME (must match POSTGRES_PASSWORD)
            CACHE_HOST: rurdesk_cache
            CACHE_PORT: "6379"
            CACHE_DB: "0"
            AI_PROVIDER: anthropic
            AI_API_KEY: ""                    # CHANGE ME (your AI provider key)
            AI_MODEL: claude-sonnet-4-6
            GIT_INTEGRATION_ENCRYPTION_KEY: "" # CHANGE ME (openssl rand -base64 32)
        depends_on:
            rurdesk.db:
                condition: service_healthy
            rurdesk.cache:
                condition: service_healthy
        networks:
            rurdesk_network:

    rurdesk.db:
        image: postgres:17
        container_name: rurdesk_db
        restart: unless-stopped
        environment:
            POSTGRES_USER: rurdesk
            POSTGRES_PASSWORD: rurdesk          # CHANGE ME
            POSTGRES_DB: rurdesk
        volumes:
            - rurdesk_data:/var/lib/postgresql/data
        healthcheck:
            test: ["CMD-SHELL", "pg_isready -U rurdesk -d rurdesk"]
            interval: 5s
            timeout: 5s
            retries: 5
        networks:
            rurdesk_network:

    rurdesk.cache:
        image: redis:8-alpine
        container_name: rurdesk_cache
        restart: unless-stopped
        command: ["redis-server"]
        volumes:
            - cache_data:/data
        healthcheck:
            test: ["CMD", "redis-cli", "ping"]
            interval: 5s
            timeout: 3s
            retries: 5
        networks:
            rurdesk_network:

    # Optional — only needed for the AI Agent workflow. Uncomment once the core
    # stack runs and you have issued the two tokens in the app.
    # gateway-goose:
    #     image: ghcr.io/bitmaster-sk/rurdesk/gateway-goose:latest
    #     restart: unless-stopped
    #     environment:
    #         LISTEN_PORT: "9090"
    #         TRACKER_URL: http://rurdesk:1000
    #         GATEWAY_TO_TRACKER_TOKEN: ""  # CHANGE ME (Admin → bot user → Bot credentials)
    #         TRACKER_TO_GATEWAY_TOKEN: ""  # CHANGE ME (… → Register gateway)
    #         REPO_URL: https://github.com/your-org/your-repo.git
    #         GIT_ACCESS_TOKEN: ""          # CHANGE ME (GitHub/GitLab PAT)
    #         GOOSE_PROVIDER: anthropic     # anthropic | google | ollama | openai
    #         ANTHROPIC_API_KEY: ""         # CHANGE ME (key for the chosen provider)
    #     volumes:
    #         - gateway-goose-workspace:/worktrees
    #         - gateway-goose-state:/var/lib/gateway
    #     depends_on:
    #         - rurdesk
    #     networks:
    #         rurdesk_network:

volumes:
    rurdesk_data:
    cache_data:
    # gateway-goose-workspace:
    # gateway-goose-state:

networks:
    rurdesk_network:
```

> **The gateway is optional and does not have to live here.** It is only needed
> for the **AI Agent workflow** — the in-API AI features (quality, kickstart,
> split) work without it. It is shown in the same compose file for convenience,
> but the gateway is a stateless executor that talks to the tracker over HTTP, so
> it runs equally well from its own compose file, on another host, or next to a
> GPU box for local Ollama. All it needs is network reach to `TRACKER_URL` and
> the tracker's webhook reach back to its `/event` endpoint.
>
> Either way, **start the core stack first**: `GATEWAY_TO_TRACKER_TOKEN` and
> `TRACKER_TO_GATEWAY_TOKEN` are issued *inside the running app*. See
> [Agent Gateway](../agentic-workflow/gateway.md).

### Secrets & where they come from

| Value | Source | How to obtain |
| --- | --- | --- |
| `POSTGRES_PASSWORD` + `DATABASE_PASSWORD` | self-chosen | Strong password, **same** value in both services |
| `GIT_INTEGRATION_ENCRYPTION_KEY` | generated | `openssl rand -base64 32` |
| `AI_API_KEY` | AI provider | Anthropic / OpenAI / Google AI Studio console; empty for local Ollama |

See [Configuration](./configuration.md) for every other variable.

## 2. Start the stack

```bash
docker compose up -d
```

| Service | Container | Notes |
| --- | --- | --- |
| `rurdesk` | `rurdesk` | API + WebSocket + MCP + Angular SPA on host port **80** (container `:1000`); runs Goose migrations on startup |
| `rurdesk.db` | `rurdesk_db` | PostgreSQL 17, data in the `rurdesk_data` volume |
| `rurdesk.cache` | `rurdesk_cache` | Redis 8 |

The tracker container applies all database migrations automatically before it
starts, so the database is initialized on first boot — there is nothing to run
by hand.

> **Recommended: run Postgres outside Docker in production.** The `rurdesk.db`
> service above is convenient for a quick start, but the official `postgres`
> image does **not** upgrade its on-disk data across major versions — starting,
> say, Postgres 18 against a data directory written by Postgres 17 fails, and
> recovering means a manual `pg_dump`/`pg_upgrade` against the volume. For
> anything you care about, point `DATABASE_HOST` at a **managed Postgres** (RDS,
> Cloud SQL, …) or a host/VM install where upgrades and backups are handled for
> you, and drop the `rurdesk.db` service. The same applies to `rurdesk.cache`
> (Redis) if you rely on its data. Keep regular backups either way.

## 3. Open the app

Navigate to **http://localhost/** (served by the tracker). The API is reachable
under **http://localhost/api/**.

![Login screen](../../site/assets/img/login.png)

## 4. First run — the one-time registration

Public registration is a **one-time bootstrap**:

1. Open `http://localhost/register` and create your account (name, e-mail,
   password). As the **first ever user you automatically become the instance
   admin** — and the registration endpoint **closes permanently**.
2. From then on, every user, bot, and admin is created from
   **Administration → Users** in the UI. See
   [User management](./user-management.md).
3. **Create your first project**, then add members or jump straight to the
   [AI Project Kickstarter](./features.md#project-kickstarter) to generate a
   backlog from a description.

## Updating

```bash
docker compose pull
docker compose up -d
```

New migrations apply automatically on tracker startup.

## Checking which version you run

Pin a release instead of `latest` (e.g. `ghcr.io/bitmaster-sk/rurdesk/rurdesk:1.0.0`)
when you want upgrades to be a deliberate step.

To see what an instance is actually running, sign in as an administrator and
open **Application settings** — the version and the short commit are shown at
the bottom of the panel.

From the host, the same identity is on the image itself:

```bash
docker inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' rurdesk
docker inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' rurdesk
```

A build that reports `dev` was built from a source checkout rather than a
tagged release.

## Next steps

- [Configuration](./configuration.md) — tune env vars and enable in-API AI features
- [Agent Gateway](../agentic-workflow/gateway.md) — add the coding-agent gateway
