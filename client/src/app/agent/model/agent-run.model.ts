import { AgentPhase } from './agent-phase.enum';

export interface AgentRun {
    idRun: number;
    idIssue: number;
    idProject: number;
    idUserBot: number;
    idGitIntegration: number | null;
    phase: AgentPhase;
    stagePlan: StagePlan;
    queuePosition: number | null;
    prUrl: string | null;
    prHostType: string | null;
    prId: string | null;
    branchName: string | null;
    errorMessage: string | null;
    approvedMockupRef?: string | null;
    startedAt: string | null;
    finishedAt: string | null;
    createdAt: string;
    events?: AgentRunEvent[];
    stages?: AgentStageProgress[];
}

export type AgentStageStatus =
    'pending' | 'active' | 'done' | 'awaiting_approval' | 'failed' | 'skipped';

export interface AgentStageProgress {
    stage: string;
    status: AgentStageStatus;
    note?: string; // semantic token: no_clarifications | submitted | pr_opened
    attemptNo?: number;
    idUserBot?: number; // which bot executed this stage (provenance)
    at?: string | null; // finishedAt (done/failed) or startedAt (active)
    approvedAt?: string | null;
    errorReason?: string | null; // stable code (AGENT.ERROR.*), set on a failed stage
    errorDetail?: string | null; // raw provider/agent message
    idResultMessage?: number | null; // the message this stage produced
    thinkingTail?: string | null; // the last thoughts of the stage
    hasThinking?: boolean; // full thinking is stored and readable
}

export interface StagePlan {
    stages: StagePlanEntry[];
}

export interface StagePlanEntry {
    name: string;
    skippable: boolean;
    skip: boolean;
    idsSkill?: number[];
}

export interface AgentRunEvent {
    idEvent: number;
    idRun: number;
    fromPhase: string | null;
    toPhase: string | null;
    actorType: 'user' | 'agent' | 'gateway' | 'system';
    idUser: number | null;
    reason: string | null;
    createdAt: string;
}

export interface RunStats {
    totalTokensUsed: number;
    totalDurationMs: number;
    totalToolCallsCount: number;
    attemptsPerStage: Record<string, number>;
    failedAttempts: number;
}

export interface CreateAgentRunReq {
    idUserBot: number;
    idsSkillByStage: Record<string, number[]>;
}
