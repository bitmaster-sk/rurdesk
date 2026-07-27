import { AgentStageProgress } from '../model/agent-run.model';

export interface FailedStageError {
    key: string; // i18n key, AGENT.ERROR.<UPPER>
    detail: string | null; // raw provider/agent message, if any
}

// Resolves the user-facing error for a failed run from its stage timeline: the
// first failed stage that carries a reason code wins. The reason code is a
// stable snake_case token (e.g. provider_credit_exhausted) mapped to an i18n key
// the template translates; an unknown code falls back to its own key, for which
// AGENT.ERROR.* provides a generic entry. Returns null when nothing failed with
// a recorded reason.
export function resolveFailedStageError(stages: AgentStageProgress[]): FailedStageError | null {
    const failed = stages.find(s => s.status === 'failed' && !!s.errorReason);
    if (!failed?.errorReason) {
        return null;
    }
    return {
        key: 'AGENT.ERROR.' + failed.errorReason.toUpperCase(),
        detail: failed.errorDetail ?? null
    };
}
