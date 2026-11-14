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

# Our license public key, passed by the release workflow to the commercial
# build only. Without it the binary reads LICENSE_PUBLIC_KEY from env, which the
# local commercial stack and e2e rely on; with it, env is ignored.
LICENSEKEY_PKG="github.com/bitmaster-sk/rurdesk/api/commercial/licensekey"
if [ -n "$EMBEDDED_LICENSE_PUBLIC_KEY" ]; then
    LDFLAGS="$LDFLAGS -X $LICENSEKEY_PKG.embeddedPublicKey=$EMBEDDED_LICENSE_PUBLIC_KEY"
fi

# Which main package becomes the `api` binary. The commercial image overrides it
# with ./commercial/cmd/api; the free default never imports commercial/.
API_PACKAGE="${API_PACKAGE:-./cmd/api}"

# Build the Go application
echo "🛠️ Start: Application build ($API_PACKAGE)"
go build -ldflags "$LDFLAGS" -o api "$API_PACKAGE"
echo "✅ Success: Application build"

# Build the admin maintenance CLI (shipped in the production image)
echo "🛠️ Start: Admin CLI build"
go build -o admin ./cmd/admin
echo "✅ Success: Admin CLI build"
