FROM golang:1.27.3-alpine AS gateway-builder
WORKDIR /build
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway/

FROM node:22-slim
# procps gives the implement-stage agent standard process inspection (pgrep, ps)
# that several git/gh flows shell out to. ripgrep is the preferred code-search
# backend; installing it gives the agent fast search in large worktrees.
# bzip2: the Goose release tarball is .tar.bz2; node:22-slim lacks bzip2, so
# `tar` extraction fails without it.
RUN apt-get update && apt-get install -y --no-install-recommends git curl ca-certificates procps ripgrep bzip2 tini && rm -rf /var/lib/apt/lists/*

# Pin the Goose CLI version so a surprise upstream release can't flip flags or
# the stream-json schema under us. GOOSE_VERSION pins the binary and the two
# checksums pin the release assets; bump all three together, after re-checking
# flags, events and exit codes against the new release. The official install
# script is not used: it falls back to the latest release when the pinned
# download fails.
ARG GOOSE_VERSION=v1.56.0
ARG GOOSE_SHA256_AMD64=e29544421a6e698aab0fe4becda96096218ff36abe85b83785519d41c555c714
ARG GOOSE_SHA256_ARM64=30b60a49a534ca7af062ec94b752367494ad3a2824744a6d1e8ec3e1040d9922
RUN case "$(dpkg --print-architecture)" in \
      amd64) goose_arch=x86_64; goose_sha256="${GOOSE_SHA256_AMD64}" ;; \
      arm64) goose_arch=aarch64; goose_sha256="${GOOSE_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac \
 && curl -fsSL -o /tmp/goose.tar.bz2 "https://github.com/aaif-goose/goose/releases/download/${GOOSE_VERSION}/goose-${goose_arch}-unknown-linux-gnu.tar.bz2" \
 && echo "${goose_sha256}  /tmp/goose.tar.bz2" | sha256sum -c - \
 && tar -xjf /tmp/goose.tar.bz2 -C /usr/local/bin ./goose \
 && rm /tmp/goose.tar.bz2 \
 && goose --version

COPY goose/plugins/rurdesk-guard /root/.agents/plugins/rurdesk-guard

COPY --from=gateway-builder /gateway /usr/local/bin/gateway
COPY common/git-hooks/pre-push /etc/git-hooks/pre-push
COPY goose/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

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
# tini must stay PID 1: the gateway does not reap orphaned agent children (esbuild, vitest), so without it they pile up as zombies.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
