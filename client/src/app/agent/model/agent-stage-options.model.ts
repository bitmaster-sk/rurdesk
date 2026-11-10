import { AgentEffort, AgentEffortSource } from './agent-effort.enum';
import { AgentStage } from './agent-stage.enum';

export interface AgentStageOptions {
    maxTurns?: number | null;
    effort?: AgentEffort | null;
}

export type AgentStageOptionsByStage = Partial<Record<AgentStage, AgentStageOptions>>;

export interface AgentStageDefault {
    maxTurns: number;
    effort: AgentEffortDefault;
}

export type AgentDefaultsByStage = Partial<Record<AgentStage, AgentStageDefault>>;

export interface AgentEffortDefault {
    effort: AgentEffort | null;
    source: AgentEffortSource;
}
