#!/bin/sh

set -e

# Run Go tests with the race detector. Pass optional args, e.g.:
# ./script/test.sh ./test/... -v
# ./script/test.sh ./... -run TestUserRegister -v
#
# -race requires cgo (gcc). The integration suite refuses to run unless
# TEST_DATABASE_* is set (api/test/setup_api_test.go: TestMain), so it can never
# touch a real database.
export CGO_ENABLED=1

# The integration suite resets the shared Postgres + Redis to a clean baseline
# before it runs (api/test/setup_api_test.go: TestMain, gated by TEST_DB_RESET,
# default on) and isolates the cache in its own Redis DB. Keep them in agreement
# by defaulting TEST_CACHE_DB here too.
export TEST_CACHE_DB="${TEST_CACHE_DB:-1}"

if [ "$#" -eq 0 ]; then
	go test ./... -race
else
	go test -race "$@"
fi
