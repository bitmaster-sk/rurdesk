import { AgentEffort } from './agent-effort.enum';
import { AgentPhase } from './agent-phase.enum';

export interface AgentRun {
    idRun: number;
    idIssue: number | null;
    idRelease: number | null;
    idProject: number;
    idUserAgent: number;
    idGitIntegration: number | null;
    phase: AgentPhase;
    stagePlan: AgentStagePlan;
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
    isParked?: boolean;
}

export type AgentStageStatus =
    'pending' | 'active' | 'done' | 'awaiting_approval' | 'failed' | 'skipped';

export interface AgentStageProgress {
    stage: string;
    status: AgentStageStatus;
    note?: string; // semantic token: no_clarifications | submitted | pr_opened
    attemptNo?: number;
    idUserAgent?: number; // which agent executed this stage (provenance)
    at?: string | null; // finishedAt (done/failed) or startedAt (active)
    approvedAt?: string | null;
    errorReason?: string | null; // stable code (AGENT.ERROR.*), set on a failed stage
    errorDetail?: string | null; // raw provider/agent message
    idResultMessage?: number | null; // the message this stage produced
    thinkingTail?: string | null; // the last thoughts of the stage
    hasThinking?: boolean; // full thinking is stored and readable
    model?: string | null;
    maxTurns?: number | null;
    effort?: AgentEffort | null;
    modelFallbackFrom?: string | null;
    attempts: number;
    tokensUsed: number;
    toolCallsCount: number;
    durationMs: number;
    inputTokens: number;
    outputTokens: number;
    cacheReadTokens: number;
    cacheWriteTokens: number;
    costUsd: number | null;
    costEstimated: boolean;
}

export interface AgentStagePlan {
    model?: string;
    effort?: AgentEffort;
    stages: AgentStagePlanEntry[];
}

export interface AgentStagePlanEntry {
    name: string;
    skippable: boolean;
    skip: boolean;
    idsSkill?: number[];
    model?: string;
    maxTurns?: number;
    effort?: AgentEffort;
}

export interface AgentRunEvent {
    idEvent: number;
    idRun: number;
    fromPhase: string | null;
    toPhase: string | null;
    actorType: 'user' | 'agent' | 'gateway' | 'system';
    idUser: number | null;
    reason: string | null;
    payload: unknown;
    createdAt: string;
}

export interface RunStats {
    totalTokensUsed: number;
    totalDurationMs: number;
    totalToolCallsCount: number;
    totalInputTokens: number;
    totalOutputTokens: number;
    totalCacheReadTokens: number;
    totalCacheWriteTokens: number;
    totalCostUsd: number | null;
    costEstimated: boolean;
    attemptsPerStage: Record<string, number>;
    failedAttempts: number;
    tokensByModel: Record<string, number>;
}

export interface AgentStatsNotice {
    idRun: number;
    stats?: RunStats;
}

export interface RunChoices {
    idsSkillByStage: Record<string, number[]>;
    isShortWorkflow: boolean;
    model?: string;
    modelByStage?: Record<string, string>;
    maxTurnsByStage?: Record<string, number>;
    effort?: AgentEffort;
    effortByStage?: Record<string, AgentEffort>;
}

export interface CreateAgentRunReq extends RunChoices {
    idUserAgent: number;
}

export interface UpdateAgentRunWorkflowReq {
    isShortWorkflow: boolean;
}
