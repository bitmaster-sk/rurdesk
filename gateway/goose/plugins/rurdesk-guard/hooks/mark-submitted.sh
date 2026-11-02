#!/bin/sh
cat >/dev/null
if [ -n "$RURDESK_STAGE_MARKER" ]; then
	touch "$RURDESK_STAGE_MARKER"
fi
