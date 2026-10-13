import { describe, expect, it } from 'vitest';
import { AgentStageProgress, AgentStageStatus, StagePlan } from '../model/agent-run.model';
import { ShortWorkflow } from './short-workflow';

function plan(skipped: string[]): StagePlan {
    return {
        stages: ['pickup', 'brainstorming', 'design', 'implementation_plan', 'implementation'].map(
            name => ({ name, skippable: true, skip: skipped.includes(name) })
        )
    };
}

function progress(statuses: Record<string, AgentStageStatus>): AgentStageProgress[] {
    return Object.entries(statuses).map(([stage, status]) => ({ stage, status }));
}

describe('ShortWorkflow.isShort', () => {
    it('is short when both design and plan are skipped', () => {
        expect(ShortWorkflow.isShort(plan(['design', 'implementation_plan']))).toBe(true);
    });

    it('is not short when only one of them is skipped', () => {
        expect(ShortWorkflow.isShort(plan(['design']))).toBe(false);
    });

    it('is not short without a plan', () => {
        expect(ShortWorkflow.isShort(null)).toBe(false);
    });
});

describe('ShortWorkflow.isLocked', () => {
    it.each([
        ['brainstorming is running', { brainstorming: 'active', design: 'pending' }, false],
        [
            'design and plan are skipped in a short run',
            { brainstorming: 'done', design: 'skipped', implementation_plan: 'skipped' },
            false
        ],
        ['the design has started', { brainstorming: 'done', design: 'active' }, true],
        ['the design failed', { brainstorming: 'done', design: 'failed' }, true],
        [
            'the implementation has started in a short run',
            { design: 'skipped', implementation_plan: 'skipped', implementation: 'active' },
            true
        ]
    ] as [string, Record<string, AgentStageStatus>, boolean][])(
        'when %s → %s',
        (_name, statuses, isLocked) => {
            expect(ShortWorkflow.isLocked(progress(statuses))).toBe(isLocked);
        }
    );
});
