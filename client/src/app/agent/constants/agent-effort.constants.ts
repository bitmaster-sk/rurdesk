import { AgentEffort } from '../model/agent-effort.enum';

export const AGENT_EFFORTS: AgentEffort[] = [
    AgentEffort.Low,
    AgentEffort.Medium,
    AgentEffort.High,
    AgentEffort.Max
];

export const AGENT_EFFORT_LABELS: Record<AgentEffort, string> = {
    [AgentEffort.Low]: 'AGENT_EFFORT.LEVEL.LOW',
    [AgentEffort.Medium]: 'AGENT_EFFORT.LEVEL.MEDIUM',
    [AgentEffort.High]: 'AGENT_EFFORT.LEVEL.HIGH',
    [AgentEffort.Max]: 'AGENT_EFFORT.LEVEL.MAX'
};
