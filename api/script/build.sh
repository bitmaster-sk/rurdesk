#!/bin/sh

set -e

# Release identity stamped into the binary. VERSION/COMMIT are passed as build
# args by the release workflow; a local build leaves them empty and the
# buildinfo defaults ("dev"/"unknown") stand, which is what we want — an
# untagged build must not claim to be a release.
BUILDINFO_PKG="github.com/bitmaster-sk/rurdesk/api/internal/buildinfo"
LDFLAGS=""
if [ -n "$VERSION" ]; then
    LDFLAGS="$LDFLAGS -X $BUILDINFO_PKG.version=$VERSION"
fi
if [ -n "$COMMIT" ]; then
    LDFLAGS="$LDFLAGS -X $BUILDINFO_PKG.commit=$COMMIT"
fi

# Build the Go application
echo "🛠️ Start: Application build"
go build -ldflags "$LDFLAGS" -o api ./cmd/api
echo "✅ Success: Application build"

# Build the admin maintenance CLI (shipped in the production image)
echo "🛠️ Start: Admin CLI build"
go build -o admin ./cmd/admin
echo "✅ Success: Admin CLI build"
