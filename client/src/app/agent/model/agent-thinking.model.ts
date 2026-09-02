import { AgentThinkingKind } from '../constants/agent-thinking-kind.enum';

export interface AgentThinkingEvent {
    kind: AgentThinkingKind;
    text?: string;
    tool?: string;
    at: number;
}

export interface AgentThinkingNotice {
    idRun: number;
    idTask: number;
    stage: string;
    seq: number;
    events: AgentThinkingEvent[];
}

export interface AgentThinkingRes {
    idRun: number;
    stage: string;
    events: AgentThinkingEvent[];
    isComplete: boolean;
}
