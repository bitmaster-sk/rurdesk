#!/bin/sh

set -e

/app/script/migrate-up.sh
exec /app/api
