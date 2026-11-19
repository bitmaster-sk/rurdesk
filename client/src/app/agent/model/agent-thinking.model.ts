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
    /** Set for a chat turn, whose notice reaches only the conversation owner. */
    idConversation?: number;
}

export interface AgentThinkingRes {
    idRun: number;
    idTask?: number;
    stage: string;
    events: AgentThinkingEvent[];
    lastSeq?: number;
    isComplete: boolean;
}
