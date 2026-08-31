package controller

import "github.com/bitmaster-sk/rurdesk/api/internal/errs"

// Controller-level error catalog: aliases of shared sentinels plus handler-specific
// domain errors. Grouped by concern (roles/users, git integration, project scoping).
var (
	errForbidden   = errs.ErrForbidden
	errCycle       = errs.ErrCycle
	errNotFound    = errs.ErrNotFound
	errLastOwner   = errs.NewErr("LAST_OWNER", "cannot remove last project owner", "error.last_owner")
	errInvalidRole = errs.NewErr("INVALID_ROLE", "invalid role value: must be viewer, member, or owner", "error.invalid_role")
	errBotOwner    = errs.NewErr("BOT_OWNER", "bot users cannot be assigned the owner role", "error.bot_owner")

	errLastAdmin          = errs.NewErr("LAST_ADMIN", "cannot remove the last instance admin", "error.last_admin")
	errBotAdmin           = errs.NewErr("BOT_ADMIN", "bot users cannot be admins", "error.bot_admin")
	errAgentActivity      = errs.NewErr("USER_HAS_AGENT_ACTIVITY", "user has agent history and cannot be deleted", "error.user_has_agent_activity")
	errAuthoredContent    = errs.NewErr("USER_HAS_AUTHORED_CONTENT", "user authored issues and cannot be deleted", "error.user_has_authored_content")
	errRegistrationClosed = errs.NewErr("REGISTRATION_CLOSED", "public registration is closed; ask an admin to create your account", "error.registration_closed")
	errMissingCredentials = errs.NewErr("MISSING_CREDENTIALS", "email and password are required for human users", "error.missing_credentials")
	errNotABot            = errs.NewErr("NOT_A_BOT", "API keys can only be managed for bot users", "error.not_a_bot")
	errApiKeyExists       = errs.NewErr("API_KEY_EXISTS", "bot already has an API key", "error.api_key_exists")
	errApiKeyLimitReached = errs.NewErr("API_KEY_LIMIT_REACHED", "user api key limit reached", "error.api_key_limit_reached")
	errApiKeySelfManage   = errs.NewErr("API_KEY_SELF_MANAGE", "api keys cannot be managed with an api key", "error.api_key_self_manage")

	errGitIntegrationNotFound  = errs.NewErr("GIT_INTEGRATION_NOT_FOUND", "git integration not found", "error.git_integration_not_found")
	errGitIntegrationDuplicate = errs.NewErr("GIT_INTEGRATION_DUPLICATE", "an integration for this repository already exists", "error.git_integration_duplicate")
	errGitHostUnavailable      = errs.NewErr("GIT_HOST_UNAVAILABLE", "git host API is unavailable", "error.git_host_unavailable")
	errInvalidMrLink           = errs.NewErr("INVALID_MR_LINK", "idGitIntegration and mrId must both be set or both be null", "error.invalid_mr_link")
	errBotNoGateway            = errs.NewErr("BOT_NO_GATEWAY", "bot user has no configured gateway", "error.bot_no_gateway")
	errStateNotInProject       = errs.NewErr("STATE_NOT_IN_PROJECT", "state does not belong to this project", "error.state_not_in_project")
	errSeverityNotInProject    = errs.NewErr("SEVERITY_NOT_IN_PROJECT", "severity does not belong to this project", "error.severity_not_in_project")
	errIssueTypeNotInProject   = errs.NewErr("ISSUE_TYPE_NOT_IN_PROJECT", "issue type does not belong to this project", "error.issue_type_not_in_project")
	errGatewayExists           = errs.NewErr("GATEWAY_EXISTS", "bot already has a gateway", "error.gateway_exists")

	errInvalidWithin = errs.NewErr("INVALID_WITHIN", `createAtWithin/updateAtWithin must be a positive duration, e.g. "30d", "2h" or "1d8h6m"`, "error.invalid_within")

	errSavedViewConfigTooLarge = errs.NewErr("SAVED_VIEW_CONFIG_TOO_LARGE", "saved view config exceeds 8KB", "error.saved_view_config_too_large")
	errSavedViewConfigInvalid  = errs.NewErr("SAVED_VIEW_CONFIG_INVALID", "saved view config is not a JSON object", "error.saved_view_config_invalid")
	errSavedViewBadSort        = errs.NewErr("SAVED_VIEW_BAD_SORT", "saved view config has an unknown orderColumn", "error.saved_view_bad_sort")
	errSavedViewLimit          = errs.NewErr("SAVED_VIEW_LIMIT", "too many saved views in this project", "error.saved_view_limit")
)
