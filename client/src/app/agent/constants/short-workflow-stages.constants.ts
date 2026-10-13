import { AgentStage } from '../model/agent-stage.enum';

export const SHORT_WORKFLOW_SKIPPED_STAGES: AgentStage[] = [
    AgentStage.Design,
    AgentStage.ImplementationPlan
];

export const SHORT_WORKFLOW_LOCKING_STAGES: AgentStage[] = [
    ...SHORT_WORKFLOW_SKIPPED_STAGES,
    AgentStage.Implementation
];
