import {
    SHORT_WORKFLOW_LOCKING_STAGES,
    SHORT_WORKFLOW_SKIPPED_STAGES
} from '../constants/short-workflow-stages.constants';
import { AgentStageProgress, StagePlan } from '../model/agent-run.model';
import { AgentStage } from '../model/agent-stage.enum';

export abstract class ShortWorkflow {
    public static isShort(plan: StagePlan | null | undefined): boolean {
        const skipped = (plan?.stages ?? []).filter(
            entry => entry.skip && SHORT_WORKFLOW_SKIPPED_STAGES.includes(entry.name as AgentStage)
        );
        return skipped.length === SHORT_WORKFLOW_SKIPPED_STAGES.length;
    }

    public static isLocked(stages: AgentStageProgress[]): boolean {
        return stages.some(
            stage =>
                SHORT_WORKFLOW_LOCKING_STAGES.includes(stage.stage as AgentStage) &&
                stage.status !== 'pending' &&
                stage.status !== 'skipped'
        );
    }
}
