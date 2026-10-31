#!/bin/sh
cat >/dev/null
if [ -z "$RURDESK_STAGE_MARKER" ] || [ -f "$RURDESK_STAGE_MARKER" ]; then
	exit 0
fi
printf '%s\n' '{"decision":"block","reason":"You have not submitted this stage yet. Call the tracker__complete_stage tool now, exactly as your instructions describe, with the id_task they give you. If you already wrote your result as a normal message, pass that text as message. If you cannot finish, call it with outcome=errored and an error_reason."}'
