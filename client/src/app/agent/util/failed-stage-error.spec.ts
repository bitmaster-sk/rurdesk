import { describe, expect, it } from 'vitest';
import { resolveFailedStageError } from './failed-stage-error';
import { AgentStageProgress } from '../model/agent-run.model';

function stage(partial: Partial<AgentStageProgress>): AgentStageProgress {
    return { stage: 'brainstorming', status: 'pending', ...partial };
}

describe('resolveFailedStageError', () => {
    it('maps a provider-credit reason to its i18n key and keeps the detail', () => {
        const result = resolveFailedStageError([
            stage({ stage: 'pickup', status: 'done' }),
            stage({
                stage: 'brainstorming',
                status: 'failed',
                errorReason: 'provider_credit_exhausted',
                errorDetail: 'Request failed: credit balance is too low'
            })
        ]);
        expect(result).toEqual({
            key: 'AGENT.ERROR.PROVIDER_CREDIT_EXHAUSTED',
            detail: 'Request failed: credit balance is too low'
        });
    });

    it('uppercases any snake_case reason into an AGENT.ERROR.* key', () => {
        const result = resolveFailedStageError([
            stage({ status: 'failed', errorReason: 'provider_error' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.PROVIDER_ERROR');
        expect(result?.detail).toBeNull();
    });

    it('maps the stage_not_submitted reason (agent exited without complete_stage)', () => {
        const result = resolveFailedStageError([
            stage({ stage: 'design', status: 'failed', errorReason: 'stage_not_submitted' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.STAGE_NOT_SUBMITTED');
    });

    it('maps the turn_limit_exhausted reason to its i18n key', () => {
        const result = resolveFailedStageError([
            stage({
                stage: 'implementation_plan',
                status: 'failed',
                errorReason: 'turn_limit_exhausted'
            })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.TURN_LIMIT_EXHAUSTED');
    });

    it('returns null when no stage failed', () => {
        expect(resolveFailedStageError([stage({ status: 'done' })])).toBeNull();
    });

    it('ignores a failed stage that carries no reason code', () => {
        expect(resolveFailedStageError([stage({ status: 'failed' })])).toBeNull();
    });

    it('picks the first failed stage that has a reason', () => {
        const result = resolveFailedStageError([
            stage({ stage: 'design', status: 'failed' }),
            stage({ stage: 'implementation', status: 'failed', errorReason: 'agent_error' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.AGENT_ERROR');
    });
});
