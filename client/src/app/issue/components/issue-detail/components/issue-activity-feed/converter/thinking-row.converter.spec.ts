import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentRun, AgentStageProgress } from 'src/app/agent/model/agent-run.model';
import { ThinkingRowConverter } from './thinking-row.converter';

function makeRun(phase: AgentPhase, stages: AgentStageProgress[] = []): AgentRun {
    return {
        idRun: 5,
        idIssue: 1,
        idProject: 10,
        idUserBot: 7,
        idGitIntegration: null,
        phase,
        stagePlan: { stages: [] },
        queuePosition: null,
        prUrl: null,
        prHostType: null,
        prId: null,
        branchName: null,
        errorMessage: null,
        startedAt: '2026-01-01T00:00:00Z',
        finishedAt: null,
        createdAt: '2026-01-01T00:00:00Z',
        stages
    };
}

describe('ThinkingRowConverter', () => {
    describe('toCurrentStage', () => {
        it('returns nothing for a run that is not in progress', () => {
            const run = makeRun(AgentPhase.AwaitingApproval, [
                { stage: 'design', status: 'active' }
            ]);

            expect(ThinkingRowConverter.toCurrentStage(run)).toBeNull();
        });

        it('returns the active stage of a running run', () => {
            const run = makeRun(AgentPhase.InProgress, [
                { stage: 'design', status: 'done' },
                { stage: 'implementation', status: 'active' }
            ]);

            expect(ThinkingRowConverter.toCurrentStage(run)?.stage).toBe('implementation');
        });

        // The row must exist from the moment the run starts, before the gateway
        // reports which stage it picked up.
        it('stands in for a running run that has no active stage yet', () => {
            const stage = ThinkingRowConverter.toCurrentStage(makeRun(AgentPhase.InProgress));

            expect(stage).toEqual({
                stage: '',
                status: 'active',
                at: '2026-01-01T00:00:00Z'
            });
        });
    });

    describe('toRowsByMessage', () => {
        it('keys the stages that recorded thinking by the message they produced', () => {
            const run = makeRun(AgentPhase.Done, [
                { stage: 'design', status: 'done', idResultMessage: 42, hasThinking: true },
                {
                    stage: 'brainstorming',
                    status: 'done',
                    idResultMessage: 42,
                    thinkingTail: 'last thoughts'
                }
            ]);

            const rows = ThinkingRowConverter.toRowsByMessage(run);

            expect(rows.get(42)?.map(stage => stage.stage)).toEqual(['design', 'brainstorming']);
        });

        it('leaves out a stage with no thinking and a stage with no message', () => {
            const run = makeRun(AgentPhase.Done, [
                { stage: 'design', status: 'done', idResultMessage: 42, hasThinking: false },
                {
                    stage: 'implementation',
                    status: 'failed',
                    idResultMessage: null,
                    hasThinking: true
                }
            ]);

            expect(ThinkingRowConverter.toRowsByMessage(run).size).toBe(0);
        });
    });

    describe('toTrailingRows', () => {
        // A failed stage posts no comment, so its thinking has nothing to hang under.
        it('collects the stages whose thinking has no comment to hang under', () => {
            const run = makeRun(AgentPhase.Failed, [
                { stage: 'design', status: 'done', idResultMessage: 42, hasThinking: true },
                {
                    stage: 'implementation',
                    status: 'failed',
                    idResultMessage: null,
                    hasThinking: true
                }
            ]);

            const rows = ThinkingRowConverter.toTrailingRows(run, null);

            expect(rows.map(row => [row.stage.stage, row.isLive])).toEqual([
                ['implementation', false]
            ]);
        });

        it('puts the current stage last and marks it live', () => {
            const current: AgentStageProgress = { stage: 'implementation', status: 'active' };
            const run = makeRun(AgentPhase.InProgress, [
                {
                    stage: 'brainstorming',
                    status: 'failed',
                    idResultMessage: null,
                    hasThinking: true
                },
                current
            ]);

            const rows = ThinkingRowConverter.toTrailingRows(run, current);

            expect(rows.map(row => [row.stage.stage, row.isLive])).toEqual([
                ['brainstorming', false],
                ['implementation', true]
            ]);
        });
    });
});
