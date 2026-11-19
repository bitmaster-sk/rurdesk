import { RunStats } from 'src/app/agent/model/agent-run.model';
import { User } from 'src/app/auth/model/user.model';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../constants/agent-conversation-message-kind.enum';
import { AgentStatusState } from '../constants/agent-status-state.enum';

export interface AgentConversation {
    idConversation: number;
    idUser: number;
    idUserAgent: number;
    idProject: number | null;
    idsProjectContext: number[];
    idRun: number | null;
    title: string;
    headCommit: string | null;
    isLocked: boolean;
    taskDraftRequest: string | null;
    readAt: string | null;
    pinnedAt: string | null;
    /** null: the gateway's default model. */
    model: string | null;
    lastAgentMessageAt: string | null;
    createdAt: string;
    updatedAt: string;
}

export interface AgentConversationListItem extends AgentConversation {
    isUnread: boolean;
    tokensUsed: number;
    costUsd: number | null;
    costEstimated: boolean;
}

export interface AgentConversationMessage {
    idMessage: number;
    idConversation: number;
    author: AgentConversationAuthor;
    kind: AgentConversationMessageKind;
    body: string;
    idUser: number | null;
    idTask: number | null;
    hasThinking: boolean;
    createdAt: string;
}

/** The agent took up a turn of the conversation. */
export interface AgentConversationTurnNotice {
    idConversation: number;
    idTask: number;
}

export interface AgentConversationProject {
    idProject: number;
    name: string;
    isMain: boolean;
}

/**
 * `busy` + `stage: "chat"` means the agent is answering this user's own turn;
 * `idTask` is then that turn's task, and null for any other work.
 */
export interface AgentStatus {
    state: AgentStatusState;
    idIssuePublic: number | null;
    idProject: number | null;
    stage: string | null;
    queuePosition: number | null;
    idTask: number | null;
}

/** What an agent's gateway offers: its models and the default one. */
export interface AgentModelChoices {
    models: string[];
    modelDefault: string | null;
}

/** An agent the owner can chat with, with the models it offers. */
export interface AgentConversationAgent extends User, AgentModelChoices {}

export interface AgentConversationDetail {
    conversation: AgentConversation;
    projects: AgentConversationProject[];
    agent: User;
    agentStatus: AgentStatus;
    agentModels: AgentModelChoices;
    stats: RunStats | null;
}

/** The model the agent answers with; null means the default. */
export interface AgentConversationSettings {
    model: string | null;
}

export interface CreateAgentConversationReq extends Partial<AgentConversationSettings> {
    idUserAgent: number;
    idProject?: number | null;
}
