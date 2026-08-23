package common

import "fmt"

// Stable error-reason codes an adapter can attach to a failed run. They flow
// through failTask → complete_stage(error_reason) → the API's agent_task row
// → the frontend, which translates via i18n (AGENT.ERROR.<UPPER>). Keep in
// sync with the client's translation keys.
const (
	// ErrCodeProviderCreditExhausted: provider rejected the request for lack
	// of credit/quota (e.g. Anthropic 400 "credit balance is too low").
	// Recoverable — user tops up and presses Continue.
	ErrCodeProviderCreditExhausted = "provider_credit_exhausted"
	// ErrCodeProviderRateLimited: provider throttled the request (HTTP 429,
	// "rate limit" / "usage limit" / "too many requests"). Transient.
	ErrCodeProviderRateLimited = "provider_rate_limited"
	// ErrCodeProviderError: some other unrecoverable provider error the agent
	// surfaced ("Ran into this error: ...").
	ErrCodeProviderError = "provider_error"
	// ErrCodeStageNotSubmitted: the agent process exited cleanly without ever
	// calling `complete_stage`, so nothing was submitted and the run would
	// otherwise hang forever — a weaker model sometimes writes its output as
	// a plain message instead of submitting it. Recoverable via Continue.
	ErrCodeStageNotSubmitted = "stage_not_submitted"
	// ErrCodeTurnLimitExhausted: the agent reached the --max-turns ceiling before
	// calling complete_stage. The process exited cleanly (exit 0 for goose), so
	// without this distinction it would be misclassified as stage_not_submitted.
	// Recoverable — press Continue to retry with a fresh turn budget.
	ErrCodeTurnLimitExhausted = "turn_limit_exhausted"
)

// AgentError is an adapter-level failure carrying a stable Code (used
// verbatim as the run's error_reason) plus a human-readable Detail. The
// orchestrator unwraps it via errors.As instead of using the generic
// "agent_error".
type AgentError struct {
	Code   string
	Detail string
}

func (e *AgentError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}
