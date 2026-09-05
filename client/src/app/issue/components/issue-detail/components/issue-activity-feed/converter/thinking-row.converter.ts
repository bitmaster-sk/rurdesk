import { AgentThinkingRow } from 'src/app/agent/entity/agent-thinking.entity';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentRun, AgentStageProgress } from 'src/app/agent/model/agent-run.model';

export abstract class ThinkingRowConverter {
    public static toCurrentStage(run: AgentRun | null): AgentStageProgress | null {
        if (run?.phase !== AgentPhase.InProgress) {
            return null;
        }
        return (
            run.stages?.find(stage => stage.status === 'active') ?? {
                stage: '',
                status: 'active',
                at: run.startedAt
            }
        );
    }

    public static toRowsByMessage(run: AgentRun | null): Map<number, AgentStageProgress[]> {
        const rows = new Map<number, AgentStageProgress[]>();
        for (const stage of run?.stages ?? []) {
            const idMessage = stage.idResultMessage;
            if (idMessage == null || !this.hasAnyThinking(stage)) {
                continue;
            }
            const existing = rows.get(idMessage);
            if (existing) {
                existing.push(stage);
            } else {
                rows.set(idMessage, [stage]);
            }
        }
        return rows;
    }

    public static toTrailingRows(
        run: AgentRun | null,
        currentStage: AgentStageProgress | null
    ): AgentThinkingRow[] {
        // A failed stage posts no comment, so its thinking has nothing to hang under.
        const rows = (run?.stages ?? [])
            .filter(
                stage =>
                    stage.idResultMessage == null &&
                    this.hasAnyThinking(stage) &&
                    stage.stage !== currentStage?.stage
            )
            .map(stage => ({ stage, isLive: false }));
        return currentStage === null ? rows : [...rows, { stage: currentStage, isLive: true }];
    }

    private static hasAnyThinking(stage: AgentStageProgress): boolean {
        return stage.hasThinking || !!stage.thinkingTail;
    }
}
