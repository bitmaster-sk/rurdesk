FROM golang:1.27.3-alpine AS gateway-builder
WORKDIR /build
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway/

FROM node:22-slim
# procps gives the implement-stage agent standard process inspection (pgrep, ps)
# that several git/gh flows shell out to. ripgrep is Claude Code's preferred code
# search backend; installing it gives the agent fast search in large worktrees.
RUN apt-get update && apt-get install -y --no-install-recommends git curl ca-certificates procps ripgrep tini && rm -rf /var/lib/apt/lists/*

# Pin the claude CLI version so a surprise upstream release can't flip flags
# under us. Bump deliberately after testing the new release locally, including the flags live
# thinking depends on (--include-partial-messages, --thinking-display summarized) and the chat
# restrictions (--tools Read,Grep,Glob,WebSearch,WebFetch, --no-session-persistence).
RUN npm install -g @anthropic-ai/claude-code@2.1.161

COPY --from=gateway-builder /gateway /usr/local/bin/gateway
COPY common/git-hooks/pre-push /etc/git-hooks/pre-push

# Copy the notice file with error handling. [t] is a workaround for Dockerfile syntax to make the COPY optional.
ARG REQUIRE_NOTICES=0
COPY THIRD_PARTY_NOTICES.tx[t] /tmp/notices/
RUN if [ -f /tmp/notices/THIRD_PARTY_NOTICES.txt ]; then \
        mkdir -p /usr/local/share/rurdesk && mv /tmp/notices/THIRD_PARTY_NOTICES.txt /usr/local/share/rurdesk/; \
    elif [ "$REQUIRE_NOTICES" = "1" ]; then \
        echo "ERROR: THIRD_PARTY_NOTICES.txt is missing — run script/licenses.sh before building this image" >&2; \
        exit 1; \
    else \
        mkdir -p /usr/local/share/rurdesk && \
        echo "Built without third-party notices. Not fit for distribution." > /usr/local/share/rurdesk/THIRD_PARTY_NOTICES.txt; \
    fi \
 && rm -rf /tmp/notices
RUN chmod +x /etc/git-hooks/pre-push

# GIT_PAGER=cat: implement-stage agent runs `git log` / `git diff` — without
# this they block on the pager and the run hangs.
ENV GIT_PAGER=cat

# Claude Code ends a Bash command after 2 min by default and caps a requested
# timeout at 10 min, too short for full builds and test suites. 30 min matches
# Goose's GOOSE_DEFAULT_EXTENSION_TIMEOUT; override either in the service env.
ENV BASH_DEFAULT_TIMEOUT_MS=1800000 \
    BASH_MAX_TIMEOUT_MS=1800000

# /root/.claude holds the Claude Console login when CLAUDE_CODE_OAUTH_TOKEN is
# not supplied — seed it once with an interactive `claude` login into the
# mounted named volume, otherwise the CLI refuses to start.
# CLAUDE_CONFIG_DIR moves the CLI state file (.claude.json) from /root into the
# same volume, so a recreated container keeps it instead of warning it is missing.
# /worktrees holds per-run git worktrees; /var/lib/gateway holds runtime state.
ENV CLAUDE_CONFIG_DIR=/root/.claude
VOLUME ["/worktrees", "/var/lib/gateway", "/root/.claude"]

EXPOSE 9090
HEALTHCHECK --interval=30s --timeout=5s CMD curl -f http://localhost:${LISTEN_PORT:-9090}/health || exit 1
# tini must stay PID 1: the gateway does not reap orphaned agent children (esbuild, vitest), so without it they pile up as zombies.
ENTRYPOINT ["/usr/bin/tini", "--", "gateway", "--adapter=claude-code"]
