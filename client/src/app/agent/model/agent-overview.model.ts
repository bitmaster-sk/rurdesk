import { AgentStage } from './agent-stage.enum';

export interface AgentCurrentRun {
    idIssuePublic: number;
    stage: AgentStage;
}

export interface AgentOverview {
    idUserBot: number;
    isBusy: boolean;
    current: AgentCurrentRun | null;
    queueCount: number;
    queuedIdsIssuePublic: number[];
    completedToday: number;
    tokens7d: number;
    avgRunDurationMs7d: number | null;
    failedAttempts7d: number;
}
