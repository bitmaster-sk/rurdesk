export interface PhaseStateMapping {
    idProject: number;
    phase: string;
    idState: number | null;
}

export interface PhaseStateMappingEntry {
    phase: string;
    idState: number | null;
}

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

export const AGENT_PHASES: AgentPhase[] = [
    AgentPhase.Queued,
    AgentPhase.InProgress,
    AgentPhase.AwaitingInput,
    AgentPhase.AwaitingApproval,
    AgentPhase.PrOpen,
    AgentPhase.Done,
    AgentPhase.Failed,
    AgentPhase.Cancelled
];

// i18n keys (resolved via the translate pipe in the template) so the mapping
// UI shares the run card's phase wording — e.g. pr_open reads "Waiting for
// merge" rather than a hardcoded "PR Open".
export const AGENT_PHASE_LABELS: Record<AgentPhase, string> = {
    [AgentPhase.Queued]: 'AGENT.PHASE.QUEUED',
    [AgentPhase.InProgress]: 'AGENT.PHASE.IN_PROGRESS',
    [AgentPhase.AwaitingInput]: 'AGENT.PHASE.AWAITING_INPUT',
    [AgentPhase.AwaitingApproval]: 'AGENT.PHASE.AWAITING_APPROVAL',
    [AgentPhase.PrOpen]: 'AGENT.PHASE.PR_OPEN',
    [AgentPhase.Done]: 'AGENT.PHASE.DONE',
    [AgentPhase.Failed]: 'AGENT.PHASE.FAILED',
    [AgentPhase.Cancelled]: 'AGENT.PHASE.CANCELLED'
};
