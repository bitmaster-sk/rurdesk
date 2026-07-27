#!/usr/bin/env bash
#
# Build the rurdesk.com static site (<repo>/site) and deploy it to a remote host
# over SSH/rsync. Mirrors site/dist -> $DEPLOY_DIR on $DEPLOY_SERVER (stale
# content is removed by --delete).
#
# Usage:
#   DEPLOY_SERVER=user@host DEPLOY_DIR=/srv/websites/rurdesk ./script/deploy-site.sh
#
# Required env:
#   DEPLOY_SERVER   SSH target, e.g. deploy@example.com  (or an ssh_config alias)
#   DEPLOY_DIR      Absolute path on the remote host to mirror the site into
#
# Both can also be set in <repo>/script/deploy-site.env (git-ignored), which is
# sourced automatically if present so credentials/paths stay out of the repo.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# This script lives in <repo>/script; the site it builds/deploys is <repo>/site.
SITE_DIR="$(cd "$SCRIPT_DIR/../site" && pwd)"

# Optional local, git-ignored config so the SSH target/path aren't baked in here.
if [[ -f "$SCRIPT_DIR/deploy-site.env" ]]; then
  # shellcheck disable=SC1091
  source "$SCRIPT_DIR/deploy-site.env"
fi

SERVER="${DEPLOY_SERVER:-}"
REMOTE_DIR="${DEPLOY_DIR:-}"

if [[ -z "$SERVER" || -z "$REMOTE_DIR" ]]; then
  cat >&2 <<'EOF'
error: DEPLOY_SERVER and DEPLOY_DIR must be set.

  DEPLOY_SERVER  SSH target, e.g. deploy@example.com (or an ssh_config alias)
  DEPLOY_DIR     absolute path on the remote host, e.g. /srv/websites/rurdesk

Set them inline:
  DEPLOY_SERVER=deploy@example.com DEPLOY_DIR=/srv/websites/rurdesk ./script/deploy-site.sh

or put them in script/deploy-site.env (git-ignored).
EOF
  exit 1
fi

echo "==> Building site (node tools/build.mjs)"
cd "$SITE_DIR"
node tools/build.mjs

echo "==> Deploying dist/ -> ${SERVER}:${REMOTE_DIR}  (mirror, stale files deleted)"
rsync -az --delete --human-readable \
  "$SITE_DIR/dist/" \
  "${SERVER}:${REMOTE_DIR}/"

echo "==> Done. New content is served immediately (static bind mount, no nginx reload needed)."
