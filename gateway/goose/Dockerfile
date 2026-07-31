FROM golang:1.25-alpine AS gateway-builder
WORKDIR /build
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway/

FROM node:22-slim
# procps gives the implement-stage agent standard process inspection (pgrep, ps)
# that several git/gh flows shell out to. ripgrep is the preferred code-search
# backend; installing it gives the agent fast search in large worktrees.
# bzip2: the Goose release tarball is .tar.bz2; node:22-slim lacks bzip2, so the
# install script's `tar` extraction fails without it.
# libgomp1: the goose binary links libgomp.so.1 (OpenMP runtime) — missing in
# node:22-slim, so `goose` won't even print --version without it.
RUN apt-get update && apt-get install -y --no-install-recommends git curl ca-certificates procps ripgrep bzip2 libgomp1 && rm -rf /var/lib/apt/lists/*

# Pin the Goose CLI version so a surprise upstream release can't flip flags or
# the stream-json schema under us. GOOSE_VERSION pins the binary; bump it
# deliberately after re-running the Task 0 checks (flags/events/exit codes)
# against the new release. (Goose moved block/goose → aaif-goose under the
# Linux Foundation; the install script still resolves from the releases host.)
ARG GOOSE_VERSION=v1.38.0
RUN curl -fsSL "https://github.com/block/goose/releases/download/stable/download_cli.sh" \
      | CONFIGURE=false GOOSE_VERSION="${GOOSE_VERSION}" bash \
 && ln -sf /root/.local/bin/goose /usr/local/bin/goose \
 && goose --version

COPY --from=gateway-builder /gateway /usr/local/bin/gateway
COPY common/git-hooks/pre-push /etc/git-hooks/pre-push
COPY goose/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Copy the notice file with error handling. [t] is a workaround for Dockerfile syntax to make the COPY optional.
COPY THIRD_PARTY_NOTICES.tx[t] /tmp/notices/
RUN test -f /tmp/notices/THIRD_PARTY_NOTICES.txt || { \
        echo "ERROR: THIRD_PARTY_NOTICES.txt is missing — run script/licenses.sh before building this image" >&2; \
        exit 1; \
    } \
 && mkdir -p /usr/local/share/rurdesk \
 && mv /tmp/notices/THIRD_PARTY_NOTICES.txt /usr/local/share/rurdesk/ \
 && rmdir /tmp/notices
RUN chmod +x /etc/git-hooks/pre-push /usr/local/bin/docker-entrypoint.sh

# GIT_PAGER=cat: implement-stage agent runs `git log` / `git diff` — without
# this they block on the pager and the run hangs.
# GOOSE_DISABLE_KEYRING=1: headless container has no OS keyring; read config and
# provider keys from env + the seeded config.yaml instead.
ENV GIT_PAGER=cat \
    GOOSE_DISABLE_KEYRING=1

# No OAuth creds volume — the goose adapter authenticates with a per-token API
# key (BYOK) passed via env (ANTHROPIC_API_KEY / GOOGLE_API_KEY / OLLAMA_HOST).
# /worktrees holds per-run git worktrees; /var/lib/gateway holds runtime state.
VOLUME ["/worktrees", "/var/lib/gateway"]

EXPOSE 9090
HEALTHCHECK --interval=30s --timeout=5s CMD curl -f http://localhost:${LISTEN_PORT:-9090}/health || exit 1

# The entrypoint seeds ~/.config/goose/config.yaml (tracker MCP extension) then
# execs the gateway in goose mode.
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
