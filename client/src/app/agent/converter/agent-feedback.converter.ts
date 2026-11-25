import { Message } from 'src/app/message/model/message.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { AgentRun } from '../model/agent-run.model';
import { AgentPhase } from '../model/agent-phase.enum';
import { AgentFeedback, AgentFeedbackBarView } from '../entity/agent-feedback.entity';
import { AgentFeedbackAction } from '../constants/agent-feedback-action.enum';

export abstract class AgentFeedbackConverter {
    // Counts people's comments and anchored replies above run.idMessageSeen. Pass every issue
    // message, not the filtered feed: a hidden comment is still unsent.
    public static toNewFeedback(messages: Message[], run: AgentRun | null): AgentFeedback {
        const feedback: AgentFeedback = { comments: 0, replies: 0, threads: 0 };
        const idMessageSeen = run?.idMessageSeen;
        if (run == null || idMessageSeen == null) {
            return feedback;
        }
        const threads = new Set<number>();
        for (const message of messages) {
            const isFeedback =
                message.messageKind === MessageKind.Comment &&
                message.idMessage > idMessageSeen &&
                !message.creator.isAgent &&
                message.creator.idUser !== run.idUserAgent;
            if (!isFeedback) {
                continue;
            }
            if (message.anchor) {
                feedback.replies++;
                threads.add(message.anchor.idThreadMessage ?? message.idMessage);
            } else {
                feedback.comments++;
            }
        }
        feedback.threads = threads.size;
        return feedback;
    }

    public static total(feedback: AgentFeedback): number {
        return feedback.comments + feedback.replies;
    }

    // Null unless the run waits for a person and its agent holds the issue. The main button is Send
    // when there is something new, else Approve of a review without mockups.
    public static toBarView(
        run: AgentRun | null,
        feedback: AgentFeedback,
        hasMockups: boolean
    ): AgentFeedbackBarView | null {
        if (!run || run.isParked || run.isAgentAssigned === false) {
            return null;
        }
        const hasNew = AgentFeedbackConverter.total(feedback) > 0;
        const action = hasNew ? AgentFeedbackAction.Send : null;
        switch (run.phase) {
            case AgentPhase.AwaitingApproval: {
                const stage = run.stages?.find(entry => entry.status === 'awaiting_approval');
                const stageKey = stage ? 'AGENT.STAGE.' + stage.stage.toUpperCase() : null;
                if (hasNew) {
                    return {
                        textKey: 'AGENT.FEEDBACK_BAR.WAITS_FOR_REVIEW',
                        stageKey,
                        action,
                        isApproveAnywayOffered: !hasMockups
                    };
                }
                if (hasMockups) {
                    return {
                        textKey: 'AGENT.FEEDBACK_BAR.CHOOSE_MOCKUP',
                        stageKey,
                        action: null,
                        isApproveAnywayOffered: false
                    };
                }
                return {
                    textKey: 'AGENT.FEEDBACK_BAR.WAITS_FOR_REVIEW',
                    stageKey,
                    action: AgentFeedbackAction.Approve,
                    isApproveAnywayOffered: false
                };
            }
            case AgentPhase.AwaitingInput:
                return {
                    textKey: 'AGENT.FEEDBACK_BAR.ASKED_QUESTION',
                    stageKey: null,
                    action,
                    isApproveAnywayOffered: false
                };
            case AgentPhase.PrOpen:
                return hasNew
                    ? {
                          textKey: 'AGENT.FEEDBACK_BAR.PR_REVIEW',
                          stageKey: null,
                          action,
                          isApproveAnywayOffered: false
                      }
                    : null;
            default:
                return null;
        }
    }
}
