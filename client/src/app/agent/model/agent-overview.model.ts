import { AgentStage } from './agent-stage.enum';

export interface AgentCurrentRun {
    idIssuePublic: number | null;
    releaseName: string | null;
    stage: AgentStage;
}

export interface AgentOverview {
    idUserAgent: number;
    isBusy: boolean;
    current: AgentCurrentRun | null;
    queueCount: number;
    queuedIdsIssuePublic: number[];
    completedToday: number;
    tokens7d: number;
    avgRunDurationMs7d: number | null;
    failedAttempts7d: number;
    models: string[];
    modelDefault: string | null;
    maxTurnsPlan: number;
    maxTurnsImplement: number;
    maxTurnsLimit: number | null;
}
