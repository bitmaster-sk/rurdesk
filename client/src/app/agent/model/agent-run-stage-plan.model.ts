import { AgentEffort } from './agent-effort.enum';
import { AgentStage } from './agent-stage.enum';
import { AgentDefaultsByStage } from './agent-stage-options.model';

export interface AgentRunStagePlanEntry {
    name: AgentStage;
    idsSkill: number[];
    model: string | null;
    maxTurns: number | null;
    effort: AgentEffort | null;
    skip: boolean;
    dispatched: boolean;
}

export interface AgentRunStagePlanRes {
    stages: AgentRunStagePlanEntry[];
    models: string[];
    modelDefault: string | null;
    maxTurnsLimit: number | null;
    defaultsByStage: AgentDefaultsByStage;
    modelsWithoutThinking: string[];
}

export interface UpdateAgentRunStagePlanEntryReq {
    stage: AgentStage;
    idsSkill: number[];
    model?: string;
    maxTurns?: number;
    effort?: AgentEffort;
}
