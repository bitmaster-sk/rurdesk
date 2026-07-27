# issue-api

Go backend for the issue tracker: REST API, WebSocket real-time notifications,
and an MCP server for LLM agents.

## Stack

Go 1.25 · Gin (HTTP) · Gorilla WebSocket · pgx + GORM (PostgreSQL) ·
Goose (migrations) · Redis · Viper (config).

## Architecture

Clean layered architecture with dependency injection (`internal/injector/`):

```
cmd/api/main.go     entry point
internal/
  router/           Gin route definitions
  controller/       HTTP request handlers
  service/          business logic
  repository/       PostgreSQL data access (pgx)
  model/            domain entities
  middleware/       auth, CORS, logging
  notify/           WebSocket real-time notifications
  mcp/              MCP server (HTTP/SSE)
  injector/         dependency injection wiring
migrations/         Goose SQL migrations
test/               integration tests
```

## Commands

```bash
./script/build.sh        # build the `api` binary into cmd/api/
./script/test.sh         # run Go tests
./script/migrate-up.sh   # run Goose migrations
go vet ./...             # static analysis
```

## Configuration

Config is read from the environment via Viper (`viper.AutomaticEnv()`). In
Docker the values come from `docker/api/.env` (copy from
[`docker/api/.env.example`](../docker/api/.env.example)).

### Environment variables

| Variable                                 | Default                             | Required           | Description                                                                                                                                                                                                                                                                                                                       |
| ---------------------------------------- | ----------------------------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `APPLICATION_HOST`                       | `0.0.0.0`                           | no                 | Listen host                                                                                                                                                                                                                                                                                                                       |
| `APPLICATION_PORT`                       | `1000`                              | no                 | Listen port. MCP is mounted on this same port under `/mcp` (no separate listener).                                                                                                                                                                                                                                                |
| `DATABASE_HOST`                          | `rurdesk-db`                        | no                 | PostgreSQL host                                                                                                                                                                                                                                                                                                                   |
| `DATABASE_NAME`                          | `rurdesk`                           | no                 | PostgreSQL database                                                                                                                                                                                                                                                                                                               |
| `DATABASE_USER`                          | `rurdesk`                           | no                 | PostgreSQL user                                                                                                                                                                                                                                                                                                                   |
| `DATABASE_PASSWORD`                      | —                                   | **yes**            | PostgreSQL password                                                                                                                                                                                                                                                                                                               |
| `CACHE_HOST`                             | `rurdesk-cache`                     | no                 | Redis host                                                                                                                                                                                                                                                                                                                        |
| `CACHE_PORT`                             | `6379`                              | no                 | Redis port                                                                                                                                                                                                                                                                                                                        |
| `CACHE_DB`                               | `0`                                 | no                 | Redis database index                                                                                                                                                                                                                                                                                                              |
| `CACHE_PASSWORD`                         | empty                               | no                 | Redis password                                                                                                                                                                                                                                                                                                                    |
| `AI_PROVIDER`                            | `ollama`                            | no                 | `anthropic` / `openai` / `gemini` / `ollama`                                                                                                                                                                                                                                                                                      |
| `AI_API_KEY`                             | —                                   | provider-dependent | API key; leave empty for Ollama                                                                                                                                                                                                                                                                                                   |
| `AI_MODEL`                               | `qwen3-coder`                       | no                 | Model id used for AI features                                                                                                                                                                                                                                                                                                     |
| `AI_HOST`                                | `http://host.docker.internal:11434` | Ollama only        | Provider base-URL override                                                                                                                                                                                                                                                                                                        |
| `AI_QUALITY_MODEL`                       | falls back to `AI_MODEL`            | no                 | Separate model for quality checks                                                                                                                                                                                                                                                                                                 |
| `AI_TIMEOUT_SECONDS`                     | provider default (`≤ 0`)            | no                 | Per-request timeout for a single AI provider call; `≤ 0` uses the provider default. Large/cloud models can take minutes.                                                                                                                                                                                                           |
| `PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH` | `10000`                             | no                 | Max project-builder description length                                                                                                                                                                                                                                                                                            |
| `WEBSOCKET_WRITE_DEADLINE`               | `10s`                               | no                 | WebSocket write timeout                                                                                                                                                                                                                                                                                                           |
| `ALLOWED_WS_ORIGINS`                     | empty                               | no                 | Extra origin hosts (comma-separated) allowed to open the WebSocket. The handshake is same-origin by default (Origin host must match the request Host); set this only for split-host deployments where the SPA is served from a different host than the API, or when a reverse proxy in front does not preserve the `Host` header. |
| `GIT_INTEGRATION_ENCRYPTION_KEY`         | —                                   | **yes**            | Encrypts stored git tokens. Generate: `openssl rand -base64 32`                                                                                                                                                                                                                                                                   |

Required values (`DATABASE_PASSWORD`, `GIT_INTEGRATION_ENCRYPTION_KEY`, and
`AI_API_KEY` for cloud AI providers) have no safe default — set them before
running in production.
