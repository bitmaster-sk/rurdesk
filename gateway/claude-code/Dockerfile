FROM golang:1.25-alpine AS gateway-builder
WORKDIR /build
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway/

FROM node:22-slim
# procps gives the implement-stage agent standard process inspection (pgrep, ps)
# that several git/gh flows shell out to. ripgrep is Claude Code's preferred code
# search backend; installing it gives the agent fast search in large worktrees.
RUN apt-get update && apt-get install -y --no-install-recommends git curl ca-certificates procps ripgrep && rm -rf /var/lib/apt/lists/*

# Pin the claude CLI version so a surprise upstream release can't flip flags
# under us. Bump deliberately after testing the new release locally.
RUN npm install -g @anthropic-ai/claude-code@2.1.161

COPY --from=gateway-builder /gateway /usr/local/bin/gateway
COPY common/git-hooks/pre-push /etc/git-hooks/pre-push

# Copy the notice file with error handling. [t] is a workaround for Dockerfile syntax to make the COPY optional.
COPY THIRD_PARTY_NOTICES.tx[t] /tmp/notices/
RUN test -f /tmp/notices/THIRD_PARTY_NOTICES.txt || { \
        echo "ERROR: THIRD_PARTY_NOTICES.txt is missing — run script/licenses.sh before building this image" >&2; \
        exit 1; \
    } \
 && mkdir -p /usr/local/share/rurdesk \
 && mv /tmp/notices/THIRD_PARTY_NOTICES.txt /usr/local/share/rurdesk/ \
 && rmdir /tmp/notices
RUN chmod +x /etc/git-hooks/pre-push

# GIT_PAGER=cat: implement-stage agent runs `git log` / `git diff` — without
# this they block on the pager and the run hangs.
ENV GIT_PAGER=cat

# /root/.claude holds the Claude subscription OAuth credentials — seed it once
# with an interactive `claude` login into the mounted named volume, otherwise
# the CLI refuses to start (unless CLAUDE_CODE_OAUTH_TOKEN is supplied).
# /worktrees holds per-run git worktrees; /var/lib/gateway holds runtime state.
VOLUME ["/worktrees", "/var/lib/gateway", "/root/.claude"]

EXPOSE 9090
HEALTHCHECK --interval=30s --timeout=5s CMD curl -f http://localhost:${LISTEN_PORT:-9090}/health || exit 1
ENTRYPOINT ["gateway", "--adapter=claude-code"]
