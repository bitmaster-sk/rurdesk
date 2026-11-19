import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentChatState } from '../constants/agent-chat-state.enum';
import { AgentStatusState } from '../constants/agent-status-state.enum';
import { AgentStatus } from '../model/agent-conversation.model';
import { AgentChatStatus } from './agent-chat-status.entity';

export abstract class AgentChatStatusConverter {
    // The ids are set only when the owner can read the project the agent works
    // in; the owner's own turn is busy too and only its stage tells it apart.
    public static toStatus(status: AgentStatus): AgentChatStatus {
        const isIssueReadable = status.idProject !== null && status.idIssuePublic !== null;
        const issue = {
            idIssuePublic: isIssueReadable ? status.idIssuePublic : null,
            issueLink: isIssueReadable
                ? ['/project', status.idProject, 'issue', status.idIssuePublic]
                : null
        };
        const none = { params: {}, idIssuePublic: null, issueLink: null };
        switch (status.state) {
            case AgentStatusState.Busy:
                if (status.stage === AgentStage.Chat) {
                    return {
                        state: AgentChatState.Answering,
                        key: 'AGENT_CHAT.STATUS.ANSWERING',
                        ...none
                    };
                }
                return {
                    state: AgentChatState.Busy,
                    key: isIssueReadable
                        ? 'AGENT_CHAT.STATUS.BUSY_ISSUE'
                        : 'AGENT_CHAT.STATUS.BUSY',
                    params: {},
                    ...issue
                };
            case AgentStatusState.Queued:
                return {
                    state: AgentChatState.Queued,
                    key: isIssueReadable
                        ? 'AGENT_CHAT.STATUS.QUEUED_ISSUE'
                        : 'AGENT_CHAT.STATUS.QUEUED',
                    params: { position: status.queuePosition ?? 1 },
                    ...issue
                };
            default:
                return { state: AgentChatState.Idle, key: 'AGENT_CHAT.STATUS.IDLE', ...none };
        }
    }
}
