export enum AgentPhase {
    Queued = 'queued',
    InProgress = 'in_progress',
    AwaitingInput = 'awaiting_input',
    AwaitingApproval = 'awaiting_approval',
    PrOpen = 'pr_open',
    Done = 'done',
    Failed = 'failed',
    Cancelled = 'cancelled'
}

export type PhaseBadgeSeverity = 'success' | 'info' | 'warn' | 'danger' | 'secondary' | 'contrast';

export const PHASE_BADGE_SEVERITY: Record<AgentPhase, PhaseBadgeSeverity> = {
    [AgentPhase.Queued]: 'secondary',
    [AgentPhase.InProgress]: 'info',
    [AgentPhase.AwaitingInput]: 'warn',
    [AgentPhase.AwaitingApproval]: 'warn',
    [AgentPhase.PrOpen]: 'contrast',
    [AgentPhase.Done]: 'success',
    [AgentPhase.Failed]: 'danger',
    [AgentPhase.Cancelled]: 'secondary'
};

export const PHASE_LABELS: Record<AgentPhase, string> = {
    [AgentPhase.Queued]: 'AGENT.PHASE.QUEUED',
    [AgentPhase.InProgress]: 'AGENT.PHASE.IN_PROGRESS',
    [AgentPhase.AwaitingInput]: 'AGENT.PHASE.AWAITING_INPUT',
    [AgentPhase.AwaitingApproval]: 'AGENT.PHASE.AWAITING_APPROVAL',
    [AgentPhase.PrOpen]: 'AGENT.PHASE.PR_OPEN',
    [AgentPhase.Done]: 'AGENT.PHASE.DONE',
    [AgentPhase.Failed]: 'AGENT.PHASE.FAILED',
    [AgentPhase.Cancelled]: 'AGENT.PHASE.CANCELLED'
};
