import { describe, expect, it } from 'vitest';
import { FailedStageErrorConverter } from './failed-stage-error.converter';
import { AgentStageProgress } from '../model/agent-run.model';

function stage(partial: Partial<AgentStageProgress>): AgentStageProgress {
    return { stage: 'brainstorming', status: 'pending', ...partial };
}

describe('FailedStageErrorConverter.toError', () => {
    it('maps a provider-credit reason to its i18n key and keeps the detail', () => {
        const result = FailedStageErrorConverter.toError([
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
        const result = FailedStageErrorConverter.toError([
            stage({ status: 'failed', errorReason: 'provider_error' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.PROVIDER_ERROR');
        expect(result?.detail).toBeNull();
    });

    it('maps the stage_not_submitted reason (agent exited without complete_stage)', () => {
        const result = FailedStageErrorConverter.toError([
            stage({ stage: 'design', status: 'failed', errorReason: 'stage_not_submitted' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.STAGE_NOT_SUBMITTED');
    });

    it('maps the turn_limit_exhausted reason to its i18n key', () => {
        const result = FailedStageErrorConverter.toError([
            stage({
                stage: 'implementation_plan',
                status: 'failed',
                errorReason: 'turn_limit_exhausted'
            })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.TURN_LIMIT_EXHAUSTED');
    });

    it('returns null when no stage failed', () => {
        expect(FailedStageErrorConverter.toError([stage({ status: 'done' })])).toBeNull();
    });

    it('ignores a failed stage that carries no reason code', () => {
        expect(FailedStageErrorConverter.toError([stage({ status: 'failed' })])).toBeNull();
    });

    it('picks the first failed stage that has a reason', () => {
        const result = FailedStageErrorConverter.toError([
            stage({ stage: 'design', status: 'failed' }),
            stage({ stage: 'implementation', status: 'failed', errorReason: 'agent_error' })
        ]);
        expect(result?.key).toBe('AGENT.ERROR.AGENT_ERROR');
    });
});
