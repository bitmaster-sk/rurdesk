#!/bin/sh
#
# Regenerates the third-party licence notices bundled into the release images,
# using `licensed` (configuration in .licensed.yml). Nothing it writes is
# committed: CI runs this before a release build, and you run it yourself
# before building a production image locally.

set -e

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

TRACKER_NOTICES="$ROOT/THIRD_PARTY_NOTICES.txt"
GATEWAY_NOTICES="$ROOT/gateway/THIRD_PARTY_NOTICES.txt"

if ! command -v licensed >/dev/null 2>&1; then
    echo "error: licensed is not installed — run \`gem install licensed\`" >&2
    exit 1
fi

# Quoted heredocs throughout: the text is emitted literally, backticks and all.

preamble_head() {
    cat <<'HEAD'
================================================================================
RURDESK — THIRD PARTY NOTICES
================================================================================

Rurdesk is distributed under the GNU Affero General Public License v3.0; see
the LICENSE file. It bundles the third-party components listed below, each of
which remains under its own licence. Those licences are reproduced here in
full, which is what the MIT, BSD, ISC and Apache-2.0 licences require of anyone
redistributing the code they cover.

This file is generated — do not edit it by hand. Everything below the preamble
is the verbatim output of `licensed` (https://github.com/licensee/licensed).
Regenerate with `script/licenses.sh`; the configuration lives in .licensed.yml.
HEAD
}

# Closing note shared by both files: applies wherever Go code is involved.
preamble_tail() {
    cat <<'TAIL'

A note on the Go sections: every Go repository published by Google ships a
PATENTS file next to its BSD-3-Clause LICENSE. That file grants patent rights
rather than imposing obligations, and both texts are reproduced below.

Build-only tooling — devDependencies, compilers, linters, test frameworks — is
out of scope: it is never distributed, so no attribution obligation attaches.
Nor does this file cover software the image installs from upstream at build
time (the Node runtime, Debian packages, the agent CLI); each of those carries
its own notices inside the image.
TAIL
}

preamble_tracker() {
    preamble_head
    cat <<'TRACKER'

SCOPE
--------------------------------------------------------------------------------
This file covers the tracker image. One section per artefact inside it, listing
the dependencies actually linked into that artefact rather than everything
named in a manifest:

  api       the server binary — REST, WebSocket, MCP, and the static SPA
  admin     the maintenance CLI shipped alongside it
  client    the Angular bundle it serves (production dependencies only)

A dependency shared by two artefacts is listed in both sections. The gateway
ships as its own image and carries its own notice file.

OBLIGATIONS BEYOND ATTRIBUTION
--------------------------------------------------------------------------------
Almost every component below asks only that this notice accompanies the
distribution. Three ask for more. Each was reviewed and accepted deliberately;
the record of that decision is the `reviewed` block in .licensed.yml.

1. github.com/hashicorp/golang-lru/v2 — MPL-2.0

   The Mozilla Public License is copyleft at the level of individual files: it
   covers the files of this component and never reaches the code combined with
   them, so Rurdesk's own source is unaffected. What it does require is that
   the source of those files stay available, at no charge and under the MPL, to
   anyone who receives a binary. Rurdesk uses the component unmodified, so the
   applicable source is the upstream release named in the section below. This
   obligation holds for the AGPL distribution and for any commercially licensed
   build alike.

2. @fontsource/lato — OFL-1.1

   The SIL Open Font License permits embedding the font in software, including
   commercial software. It requires that the licence and copyright notice
   travel with the font (this file), that the font is not sold on its own, and
   that a modified font is not distributed under the original reserved name.

3. emoji-toolkit — MIT for code, proprietary terms for artwork

   Despite the `MIT` in its package metadata, JoyPixels licenses this package
   in two halves: the JavaScript, JSON and CSS under MIT, and the emoji
   *artwork* under its own restrictive terms, which require a paid plan for
   several commercial uses.

   Rurdesk ships only lib/js/joypixels.min.js (see client/angular.json), and
   uses it solely to turn shortnames such as :smile: into Unicode codepoints,
   which the reader's own system font draws. No JoyPixels artwork is bundled,
   referenced or served, so only the MIT half applies. Revisit this if the
   emoji pipeline ever switches to image-based emoji.
TRACKER
    preamble_tail
}

preamble_gateway() {
    preamble_head
    cat <<'GATEWAY'

SCOPE
--------------------------------------------------------------------------------
This file covers the gateway images (rurdesk-gateway-claude and
rurdesk-gateway-goose), which ship the `gateway` binary built from the separate
Go module in this directory. The tracker image ships different artefacts and
carries its own notice file at the repository root.

OBLIGATIONS BEYOND ATTRIBUTION
--------------------------------------------------------------------------------
None. Every component below is under a permissive licence whose only
requirement is that this notice accompanies the distribution.
GATEWAY
    preamble_tail

    # The gateway images build from the ./gateway context and so cannot reach
    # the LICENSE at the repository root, yet the AGPL text has to travel with
    # them. Carrying it inside this generated file beats keeping a second copy
    # of LICENSE checked into gateway/.
    cat <<'AGPL_HEAD'


################################################################################
# Rurdesk's own licence
################################################################################

AGPL_HEAD
    cat "$ROOT/LICENSE"
}

# assemble <target file> <preamble function> <app>...
assemble() {
    target=$1
    render_preamble=$2
    shift 2

    "$render_preamble" > "$target"
    for app in "$@"; do
        src="$ROOT/.licenses/$app/NOTICE"
        if [ ! -f "$src" ]; then
            echo "error: missing $src — run \`licensed notices\` first" >&2
            exit 1
        fi
        {
            printf '\n\n'
            printf '################################################################################\n'
            printf '# %s\n' "$app"
            printf '################################################################################\n\n'
            cat "$src"
        } >> "$target"
    done
}

assemble_all() {
    assemble "$1" preamble_tracker api admin client
    assemble "$2" preamble_gateway gateway
}

licensed cache
licensed status
licensed notices
assemble_all "$TRACKER_NOTICES" "$GATEWAY_NOTICES"

echo "wrote THIRD_PARTY_NOTICES.txt and gateway/THIRD_PARTY_NOTICES.txt"
