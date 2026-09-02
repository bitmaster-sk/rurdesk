package errs

import (
	"errors"
	"net/http"
)

// Error is a typed application error carrying a code, message, i18n translate key,
// and HTTP status so controllers can respond consistently.
type Error struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	TranslateKey string `json:"translateKey,omitempty"`
	status       int
}

func (e *Error) Error() string { return e.Message }

func (e *Error) HttpStatus() int { return e.status }

// WithTranslateKey returns a copy of the error with the given translate key set.
// Use this to attach a dynamic key without mutating the shared sentinel.
func (e *Error) WithTranslateKey(key string) *Error {
	cp := *e
	cp.TranslateKey = key
	return &cp
}

func (e *Error) WithMessage(message string) *Error {
	cp := *e
	cp.Message = message
	return &cp
}

func newErr(code, message, translateKey string, status int) *Error {
	return &Error{Code: code, Message: message, TranslateKey: translateKey, status: status}
}

// NewErr creates a new application error with StatusUnprocessableEntity by default.
// Use for domain-level errors that don't fit existing sentinels.
func NewErr(code, message, translateKey string) *Error {
	return newErr(code, message, translateKey, http.StatusUnprocessableEntity)
}

var (
	// ── Generic HTTP sentinels ──────────────────────────────────────────────

	ErrBadRequest   = newErr("BAD_REQUEST", "bad request", "error.bad_request", http.StatusBadRequest)
	ErrValidation   = newErr("VALIDATION_FAILED", "request validation failed", "error.validation", http.StatusBadRequest)
	ErrUnauthorized = newErr("UNAUTHORIZED", "unauthorized", "error.unauthorized", http.StatusUnauthorized)
	ErrForbidden    = newErr("FORBIDDEN", "forbidden", "error.forbidden", http.StatusForbidden)
	ErrNotFound     = newErr("NOT_FOUND", "not found", "error.not_found", http.StatusNotFound)
	ErrInternal     = newErr("INTERNAL_ERROR", "internal server error", "error.internal", http.StatusInternalServerError)
	ErrConflict     = newErr("CONFLICT", "conflict", "error.conflict", http.StatusConflict)

	// ── Workflow / state / severity / issue-type ────────────────────────────

	ErrStateInUse         = newErr("STATE_IN_USE", "state is still in use (issues, project default or agent phases); pass migrateTo=<id> or migrateTo=null", "error.state_in_use", http.StatusConflict)
	ErrSeverityInUse      = newErr("SEVERITY_IN_USE", "severity is still in use (issues or project default); pass migrateTo=<id> or migrateTo=null", "error.severity_in_use", http.StatusConflict)
	ErrIssueTypeInUse     = newErr("ISSUE_TYPE_IN_USE", "issue type is still in use (issues or project default); pass migrateTo=<id> or migrateTo=null", "error.issue_type_in_use", http.StatusConflict)
	ErrIssueTypeProtected = newErr("ISSUE_TYPE_PROTECTED", "issue type is protected and cannot be deleted", "error.issue_type_protected", http.StatusConflict)

	ErrInvalidStateMigrationTarget     = newErr("INVALID_MIGRATION_TARGET", "migration target must be a different state of the same project", "error.invalid_state_migration_target", http.StatusUnprocessableEntity)
	ErrInvalidSeverityMigrationTarget  = newErr("INVALID_MIGRATION_TARGET", "migration target must be a different severity of the same project", "error.invalid_severity_migration_target", http.StatusUnprocessableEntity)
	ErrInvalidIssueTypeMigrationTarget = newErr("INVALID_MIGRATION_TARGET", "migration target must be a different issue type of the same project", "error.invalid_issue_type_migration_target", http.StatusUnprocessableEntity)

	// ── Issue relations ────────────────────────────────────────────────────

	ErrCycle         = newErr("CYCLE", "relation would create a cycle", "error.relation_cycle", http.StatusUnprocessableEntity)
	ErrInvalidWithin = newErr("INVALID_WITHIN", `createAtWithin/updateAtWithin must be a positive duration, e.g. "30d", "2h" or "1d8h6m"`, "error.invalid_within", http.StatusUnprocessableEntity)

	// ── AI / rate limiting ──────────────────────────────────────────────────

	ErrAiUnavailable     = newErr("AI_UNAVAILABLE", "AI service unavailable", "error.ai_unavailable", http.StatusServiceUnavailable)
	ErrAiNotConfigured   = newErr("AI_NOT_CONFIGURED", "no AI model configured", "error.ai_not_configured", http.StatusServiceUnavailable)
	ErrAiInvalidResponse = newErr("AI_INVALID_RESPONSE", "AI returned an invalid response", "error.ai_invalid_response", http.StatusUnprocessableEntity)
	ErrRateLimited       = newErr("RATE_LIMITED", "too many requests", "error.rate_limited", http.StatusTooManyRequests)

	// ── Sprint ─────────────────────────────────────────────────────────────

	ErrSprintWindow = newErr("SPRINT_WINDOW", "sprint must end after it starts", "error.sprint_window", http.StatusUnprocessableEntity)
	ErrSprintClosed = newErr("SPRINT_CLOSED", "sprint is already closed", "error.sprint_closed", http.StatusConflict)

	// ── Skill ──────────────────────────────────────────────────────────────

	ErrSkillNotFound        = newErr("SKILL_NOT_FOUND", "skill not found", "error.skill_not_found", http.StatusNotFound)
	ErrSkillNameTaken       = newErr("SKILL_NAME_TAKEN", "a skill with this name already exists", "error.skill_name_taken", http.StatusConflict)
	ErrSkillBuiltin         = newErr("SKILL_BUILTIN", "builtin skills cannot be deleted", "error.skill_builtin", http.StatusConflict)
	ErrSkillNotBuiltin      = newErr("SKILL_NOT_BUILTIN", "custom skills have no shipped original to restore", "error.skill_not_builtin", http.StatusConflict)
	ErrSkillStageDispatched = newErr("SKILL_STAGE_DISPATCHED", "stage already started; its skills can no longer be changed", "error.skill_stage_dispatched", http.StatusConflict)
	ErrUnknownSkill         = newErr("UNKNOWN_SKILL", "the request references a skill that does not exist", "error.unknown_skill", http.StatusBadRequest)
	ErrStageNotInPlan       = newErr("STAGE_NOT_IN_PLAN", "stage is not in the run's stage plan", "error.stage_not_in_plan", http.StatusBadRequest)

	// ── Roles / users ──────────────────────────────────────────────────────

	ErrLastOwner     = newErr("LAST_OWNER", "cannot remove last project owner", "error.last_owner", http.StatusUnprocessableEntity)
	ErrInvalidRole   = newErr("INVALID_ROLE", "invalid role value: must be viewer, member, or owner", "error.invalid_role", http.StatusUnprocessableEntity)
	ErrBotOwner      = newErr("BOT_OWNER", "bot users cannot be assigned the owner role", "error.bot_owner", http.StatusUnprocessableEntity)
	ErrLastAdmin     = newErr("LAST_ADMIN", "cannot remove the last instance admin", "error.last_admin", http.StatusUnprocessableEntity)
	ErrBotAdmin      = newErr("BOT_ADMIN", "bot users cannot be admins", "error.bot_admin", http.StatusUnprocessableEntity)
	ErrAgentActivity = newErr("USER_HAS_AGENT_ACTIVITY", "user has agent history and cannot be deleted", "error.user_has_agent_activity", http.StatusUnprocessableEntity)

	ErrAuthoredContent    = newErr("USER_HAS_AUTHORED_CONTENT", "user authored issues and cannot be deleted", "error.user_has_authored_content", http.StatusUnprocessableEntity)
	ErrRegistrationClosed = newErr("REGISTRATION_CLOSED", "public registration is closed; ask an admin to create your account", "error.registration_closed", http.StatusUnprocessableEntity)
	ErrMissingCredentials = newErr("MISSING_CREDENTIALS", "email and password are required for human users", "error.missing_credentials", http.StatusBadRequest)

	// ── API keys ───────────────────────────────────────────────────────────

	ErrNotABot            = newErr("NOT_A_BOT", "API keys can only be managed for bot users", "error.not_a_bot", http.StatusUnprocessableEntity)
	ErrApiKeyExists       = newErr("API_KEY_EXISTS", "bot already has an API key", "error.api_key_exists", http.StatusUnprocessableEntity)
	ErrApiKeyLimitReached = newErr("API_KEY_LIMIT_REACHED", "user api key limit reached", "error.api_key_limit_reached", http.StatusUnprocessableEntity)
	ErrApiKeySelfManage   = newErr("API_KEY_SELF_MANAGE", "api keys cannot be managed with an api key", "error.api_key_self_manage", http.StatusUnprocessableEntity)

	// ── Git integration ────────────────────────────────────────────────────

	ErrGitIntegrationNotFound  = newErr("GIT_INTEGRATION_NOT_FOUND", "git integration not found", "error.git_integration_not_found", http.StatusNotFound)
	ErrGitIntegrationDuplicate = newErr("GIT_INTEGRATION_DUPLICATE", "an integration for this repository already exists", "error.git_integration_duplicate", http.StatusConflict)
	ErrGitHostUnavailable      = newErr("GIT_HOST_UNAVAILABLE", "git host API is unavailable", "error.git_host_unavailable", http.StatusServiceUnavailable)
	ErrInvalidMrLink           = newErr("INVALID_MR_LINK", "idGitIntegration and mrId must both be set or both be null", "error.invalid_mr_link", http.StatusUnprocessableEntity)
	ErrBotNoGateway            = newErr("BOT_NO_GATEWAY", "bot user has no configured gateway", "error.bot_no_gateway", http.StatusUnprocessableEntity)
	ErrGatewayExists           = newErr("GATEWAY_EXISTS", "bot already has a gateway", "error.gateway_exists", http.StatusConflict)

	// ── Project scoping ────────────────────────────────────────────────────

	ErrStateNotInProject     = newErr("STATE_NOT_IN_PROJECT", "state does not belong to this project", "error.state_not_in_project", http.StatusUnprocessableEntity)
	ErrSeverityNotInProject  = newErr("SEVERITY_NOT_IN_PROJECT", "severity does not belong to this project", "error.severity_not_in_project", http.StatusUnprocessableEntity)
	ErrIssueTypeNotInProject = newErr("ISSUE_TYPE_NOT_IN_PROJECT", "issue type does not belong to this project", "error.issue_type_not_in_project", http.StatusUnprocessableEntity)

	// ── Saved views ────────────────────────────────────────────────────────

	ErrSavedViewConfigTooLarge = newErr("SAVED_VIEW_CONFIG_TOO_LARGE", "saved view config exceeds 8KB", "error.saved_view_config_too_large", http.StatusUnprocessableEntity)
	ErrSavedViewConfigInvalid  = newErr("SAVED_VIEW_CONFIG_INVALID", "saved view config is not a JSON object", "error.saved_view_config_invalid", http.StatusUnprocessableEntity)
	ErrSavedViewBadSort        = newErr("SAVED_VIEW_BAD_SORT", "saved view config has an unknown orderColumn", "error.saved_view_bad_sort", http.StatusUnprocessableEntity)
	ErrSavedViewLimit          = newErr("SAVED_VIEW_LIMIT", "too many saved views in this project", "error.saved_view_limit", http.StatusUnprocessableEntity)

	// ── Agent run lifecycle ────────────────────────────────────────────────

	ErrRunNotAwaitingApproval = newErr("RUN_NOT_AWAITING_APPROVAL", "run is not awaiting approval", "error.run_not_awaiting_approval", http.StatusUnprocessableEntity)
	ErrPhaseMismatch          = newErr("PHASE_MISMATCH", "phase mismatch", "error.phase_mismatch", http.StatusUnprocessableEntity)
	ErrRunTerminal            = newErr("RUN_TERMINAL", "run is already in a terminal phase", "error.run_terminal", http.StatusUnprocessableEntity)
	ErrRunContinueInvalid     = newErr("RUN_CONTINUE_INVALID", "continue only valid on failed or cancelled runs", "error.run_continue_invalid", http.StatusUnprocessableEntity)
	ErrNoStageToContinue      = newErr("NO_STAGE_TO_CONTINUE", "no stage to continue", "error.no_stage_to_continue", http.StatusUnprocessableEntity)
	ErrRunHasPr               = newErr("RUN_HAS_PR", "cannot restart a run that already has a pull request; continue or close it instead", "error.run_has_pr", http.StatusUnprocessableEntity)
	ErrTaskNotFound           = newErr("TASK_NOT_FOUND", "task not found", "error.task_not_found", http.StatusUnprocessableEntity)
	ErrTaskNotRunningByAgent  = newErr("TASK_NOT_RUNNING_BY_AGENT", "agent task is not being executed by an agent", "error.task_not_running_by_agent", http.StatusConflict)

	// ── Issue / message conflicts ──────────────────────────────────────────

	ErrIssueHasActiveRun     = newErr("ISSUE_HAS_ACTIVE_RUN", "issue already has an active run; hand it over by changing the assignee instead", "error.issue_has_active_run", http.StatusUnprocessableEntity)
	ErrBotPostWhileRunPaused = newErr("BOT_POST_WHILE_RUN_PAUSED", "bot cannot post while run is paused", "error.bot_post_while_run_paused", http.StatusUnprocessableEntity)

	// ── Project builder ────────────────────────────────────────────────────

	ErrInvalidProjectBuilderRefs = newErr("INVALID_PROJECT_BUILDER_REFS", "invalid cross-references in project builder payload", "error.invalid_project_builder_refs", http.StatusUnprocessableEntity)
)

// As reports whether err is (or wraps) an *Error, storing it in target. It
// delegates to errors.As so a sentinel stays matchable after being wrapped with %w.
func As(err error, target **Error) bool {
	return errors.As(err, target)
}

// FromStatus returns the sentinel *Error for a bare HTTP status, so a handler that
// set only a status (no typed error) still yields a translatable body. Unmapped
// statuses fall back to ErrInternal.
func FromStatus(status int) *Error {
	switch status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusServiceUnavailable:
		return ErrAiUnavailable
	default:
		return ErrInternal
	}
}
