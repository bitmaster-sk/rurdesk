import { describe, expect, it } from 'vitest';
import { AgentStageCardConverter } from './agent-stage-card.converter';
import { AgentStageCardBody, AgentStageCardSource } from '../entity/agent-stage-card.entity';
import { Fixtures } from 'src/testing/fixtures';

function source(partial: Partial<AgentStageCardSource>): AgentStageCardSource {
    return {
        stages: [],
        plan: [],
        runModel: 'openai/kimi',
        defaultMaxTurns: () => 40,
        skillNames: new Map([
            [1, 'go-backend'],
            [2, 'angular-client']
        ]),
        agentNames: new Map(),
        isEditable: true,
        ...partial
    };
}

describe('AgentStageCardConverter.toViews', () => {
    it('tags a not-started stage with its skills and only the settings that differ from the run', () => {
        const [overridden, inherited] = AgentStageCardConverter.toViews(
            source({
                stages: [
                    Fixtures.stageProgress({ stage: 'implementation_plan' }),
                    Fixtures.stageProgress({ stage: 'implementation' })
                ],
                plan: [
                    {
                        name: 'implementation_plan',
                        idsSkill: [1, 2],
                        model: 'anthropic/sonnet',
                        maxTurns: 80,
                        dispatched: false
                    },
                    {
                        name: 'implementation',
                        idsSkill: [1],
                        model: 'openai/kimi',
                        maxTurns: 40,
                        dispatched: false
                    }
                ]
            })
        );

        expect(overridden.planTags).toEqual({
            skillsCount: 2,
            model: 'anthropic/sonnet',
            maxTurns: 80
        });
        expect(inherited.planTags).toEqual({ skillsCount: 1, model: null, maxTurns: null });
    });

    it('opens the stage waiting for review and the next stage to start', () => {
        const views = AgentStageCardConverter.toViews(
            source({
                stages: [
                    Fixtures.stageProgress({ stage: 'brainstorming', status: 'done', attempts: 1 }),
                    Fixtures.stageProgress({
                        stage: 'design',
                        status: 'awaiting_approval',
                        attempts: 1
                    }),
                    Fixtures.stageProgress({ stage: 'implementation_plan' }),
                    Fixtures.stageProgress({ stage: 'implementation' })
                ],
                plan: ['brainstorming', 'design', 'implementation_plan', 'implementation'].map(
                    name => ({ name, idsSkill: [], model: null, maxTurns: null, dispatched: false })
                )
            })
        );

        expect(views.map(view => view.isOpenByDefault)).toEqual([false, true, true, false]);
    });

    it('gives a skipped stage no body, so it cannot be opened', () => {
        const [skipped] = AgentStageCardConverter.toViews(
            source({
                stages: [Fixtures.stageProgress({ stage: 'design', status: 'skipped' })],
                plan: [
                    {
                        name: 'design',
                        idsSkill: [1],
                        model: null,
                        maxTurns: null,
                        dispatched: false
                    }
                ]
            })
        );

        expect(skipped.body).toBe(AgentStageCardBody.None);
        expect(skipped.isOpenByDefault).toBe(false);
    });

    it('shows a stage that ran with its usage and skills, and never as a plan', () => {
        const [ran] = AgentStageCardConverter.toViews(
            source({
                stages: [
                    Fixtures.stageProgress({
                        stage: 'design',
                        status: 'done',
                        attempts: 2,
                        tokensUsed: 1200,
                        toolCallsCount: 9,
                        model: 'openai/kimi',
                        maxTurns: 40
                    })
                ],
                plan: [
                    {
                        name: 'design',
                        idsSkill: [2, 99],
                        model: null,
                        maxTurns: null,
                        dispatched: false
                    }
                ]
            })
        );

        expect(ran.body).toBe(AgentStageCardBody.Ran);
        expect(ran.planTags).toBeNull();
        expect(ran).toMatchObject({
            attempts: 2,
            tokensUsed: 1200,
            toolCallsCount: 9,
            model: 'openai/kimi',
            maxTurns: 40,
            skillNames: ['angular-client']
        });
    });

    it('lets the user edit a stage again after its only attempt was cancelled', () => {
        const [cancelled] = AgentStageCardConverter.toViews(
            source({
                stages: [Fixtures.stageProgress({ stage: 'design', attempts: 1 })],
                plan: [
                    { name: 'design', idsSkill: [], model: null, maxTurns: null, dispatched: false }
                ]
            })
        );

        expect(cancelled.body).toBe(AgentStageCardBody.Plan);
        expect(cancelled.isOpenByDefault).toBe(true);
    });

    it('keeps a stage that already ran out of the editor even when its last attempt was cancelled', () => {
        const [rerun] = AgentStageCardConverter.toViews(
            source({
                stages: [Fixtures.stageProgress({ stage: 'design', attempts: 2 })],
                plan: [
                    { name: 'design', idsSkill: [], model: null, maxTurns: null, dispatched: true }
                ]
            })
        );

        expect(rerun.body).toBe(AgentStageCardBody.Ran);
    });

    it('offers no editor once the run has ended', () => {
        const [pending] = AgentStageCardConverter.toViews(
            source({
                isEditable: false,
                stages: [Fixtures.stageProgress({ stage: 'implementation' })],
                plan: [
                    {
                        name: 'implementation',
                        idsSkill: [],
                        model: null,
                        maxTurns: null,
                        dispatched: false
                    }
                ]
            })
        );

        expect(pending.body).toBe(AgentStageCardBody.None);
        expect(pending.isOpenByDefault).toBe(false);
    });

    it('names the agent of each stage only after a hand-off', () => {
        const agentNames = new Map([
            [7, 'Coder'],
            [8, 'Planner']
        ]);
        const single = AgentStageCardConverter.toViews(
            source({
                agentNames,
                stages: [Fixtures.stageProgress({ status: 'done', attempts: 1, idUserAgent: 7 })]
            })
        );
        const handedOff = AgentStageCardConverter.toViews(
            source({
                agentNames,
                stages: [
                    Fixtures.stageProgress({
                        stage: 'brainstorming',
                        status: 'done',
                        attempts: 1,
                        idUserAgent: 8
                    }),
                    Fixtures.stageProgress({
                        stage: 'design',
                        status: 'done',
                        attempts: 1,
                        idUserAgent: 7
                    })
                ]
            })
        );

        expect(single[0].agentName).toBeNull();
        expect(handedOff.map(view => view.agentName)).toEqual(['Planner', 'Coder']);
    });
});
