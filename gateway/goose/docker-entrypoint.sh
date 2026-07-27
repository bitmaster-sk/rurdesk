#!/bin/sh
# Entrypoint for the goose gateway. The tracker MCP config.yaml is written by the
# adapter per run (writeGooseConfig) with the concrete stage-scoped URL + bearer
# token — goose doesn't substitute ${ENV} in an extension uri, and the URL
# differs by stage — so nothing is seeded here. We only disable the OS keyring
# (headless) and exec the gateway.
set -e

export GOOSE_DISABLE_KEYRING=1

exec gateway --adapter=goose
