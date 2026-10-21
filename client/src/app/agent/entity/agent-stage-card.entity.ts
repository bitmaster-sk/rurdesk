import { AgentStageProgress, AgentStageStatus } from '../model/agent-run.model';

export enum AgentStageCardBody {
    Ran = 'ran',
    Plan = 'plan',
    None = 'none'
}

export interface AgentStageCardPlanTags {
    skillsCount: number;
    model: string | null;
    maxTurns: number | null;
}

export interface AgentStageCardView {
    stage: string;
    label: string;
    status: AgentStageStatus;
    statusKey: string;
    glyph: string;
    agentName: string | null;
    at: string | null;
    approvedAt: string | null;
    attempts: number;
    tokensUsed: number;
    toolCallsCount: number;
    durationMs: number;
    model: string | null;
    modelFallbackFrom: string | null;
    maxTurns: number | null;
    skillNames: string[];
    planTags: AgentStageCardPlanTags | null;
    body: AgentStageCardBody;
    isOpenByDefault: boolean;
}

export interface AgentStageCardPlanEntry {
    name: string;
    idsSkill: number[];
    model: string | null;
    maxTurns: number | null;
    dispatched: boolean;
}

export interface AgentStageCardSource {
    stages: AgentStageProgress[];
    plan: AgentStageCardPlanEntry[];
    runModel: string | null;
    defaultMaxTurns: (stage: string) => number | null;
    skillNames: Map<number, string>;
    agentNames: Map<number, string>;
    isEditable: boolean;
}
