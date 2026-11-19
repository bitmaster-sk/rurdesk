import { AgentChatState } from '../constants/agent-chat-state.enum';

/** What the conversation header says about the agent. */
export interface AgentChatStatus {
    state: AgentChatState;
    key: string;
    params: Record<string, number>;
    /** The issue the agent works on, only when the owner can read its project. */
    idIssuePublic: number | null;
    issueLink: (string | number | null)[] | null;
}
