import { AgentStage } from './agent-stage.enum';

export interface AgentRunStagePlanEntry {
    name: AgentStage;
    idsSkill: number[];
    model: string | null;
    maxTurns: number | null;
    skip: boolean;
    dispatched: boolean;
}

export interface AgentRunStagePlanRes {
    stages: AgentRunStagePlanEntry[];
    models: string[];
    modelDefault: string | null;
    maxTurnsPlan: number;
    maxTurnsImplement: number;
    maxTurnsLimit: number | null;
}

export interface UpdateAgentRunStagePlanEntryReq {
    stage: AgentStage;
    idsSkill: number[];
    model?: string;
    maxTurns?: number;
}
