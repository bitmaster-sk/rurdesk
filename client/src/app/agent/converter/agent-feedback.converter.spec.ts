import { describe, expect, it } from 'vitest';
import { Fixtures } from 'src/testing/fixtures';
import { Message } from 'src/app/message/model/message.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { AgentPhase } from '../model/agent-phase.enum';
import { AgentFeedbackConverter } from './agent-feedback.converter';
import { AgentFeedbackAction } from '../constants/agent-feedback-action.enum';

const person = Fixtures.user({ idUser: 1 });
const agent = Fixtures.agent({ idUser: 8 });

function message(idMessage: number, overrides: Partial<Message> = {}): Message {
    return {
        idMessage,
        message: 'text',
        messageKind: MessageKind.Comment,
        createdAt: new Date(idMessage),
        isRead: true,
        creator: person,
        idRecipient: 10,
        idMessageRecipientType: MessageRecipientType.issue,
        version: 1,
        anchor: null,
        ...overrides
    };
}

function reply(idMessage: number, idThread: number): Message {
    return message(idMessage, {
        anchor: {
            idParentMessage: 5,
            parentVersion: 1,
            anchorLineStart: 1,
            anchorLineEnd: 1,
            isOutdated: false,
            anchorText: 'line',
            idThreadMessage: idThread === idMessage ? undefined : idThread
        }
    });
}

describe('AgentFeedbackConverter.toNewFeedback', () => {
    const run = Fixtures.agentRun({ idUserAgent: agent.idUser, idMessageSeen: 10 });

    it('counts people comments and replies above the watermark, threads once each', () => {
        const feedback = AgentFeedbackConverter.toNewFeedback(
            [message(9), message(11), reply(12, 12), reply(13, 12), reply(14, 14)],
            run
        );
        expect(feedback).toEqual({ comments: 1, replies: 3, threads: 2 });
    });

    it('ignores agent messages and agent outputs', () => {
        const feedback = AgentFeedbackConverter.toNewFeedback(
            [
                message(11, { creator: agent }),
                message(12, { creator: Fixtures.agent({ idUser: 99 }) }),
                message(13, { messageKind: MessageKind.ImplementationPlan })
            ],
            run
        );
        expect(AgentFeedbackConverter.total(feedback)).toBe(0);
    });

    it('counts nothing before an attempt completed', () => {
        const feedback = AgentFeedbackConverter.toNewFeedback(
            [message(11)],
            Fixtures.agentRun({ idMessageSeen: null })
        );
        expect(AgentFeedbackConverter.total(feedback)).toBe(0);
    });

    it('counts every comment when the attempt saw none', () => {
        const feedback = AgentFeedbackConverter.toNewFeedback(
            [message(1), message(2)],
            Fixtures.agentRun({ idMessageSeen: 0 })
        );
        expect(feedback.comments).toBe(2);
    });
});

describe('AgentFeedbackConverter.toBarView', () => {
    const none = { comments: 0, replies: 0, threads: 0 };
    const some = { comments: 1, replies: 0, threads: 0 };
    const waitingPlan = Fixtures.agentRun({
        phase: AgentPhase.AwaitingApproval,
        stages: [
            Fixtures.stageProgress({ stage: 'implementation_plan', status: 'awaiting_approval' })
        ]
    });

    it('offers Send while new comments wait, Approve when nothing is new', () => {
        expect(AgentFeedbackConverter.toBarView(waitingPlan, some, false)).toEqual({
            textKey: 'AGENT.FEEDBACK_BAR.WAITS_FOR_REVIEW',
            stageKey: 'AGENT.STAGE.IMPLEMENTATION_PLAN',
            action: AgentFeedbackAction.Send,
            isApproveAnywayOffered: true
        });
        expect(AgentFeedbackConverter.toBarView(waitingPlan, none, false)?.action).toBe(
            AgentFeedbackAction.Approve
        );
    });

    it('leaves a design with mockups to its mockup cards', () => {
        const view = AgentFeedbackConverter.toBarView(waitingPlan, none, true);
        expect(view?.textKey).toBe('AGENT.FEEDBACK_BAR.CHOOSE_MOCKUP');
        expect(view?.action).toBeNull();
    });

    it('shows a question without a button until there is an answer to send', () => {
        const asking = Fixtures.agentRun({ phase: AgentPhase.AwaitingInput });
        expect(AgentFeedbackConverter.toBarView(asking, none, false)?.action).toBeNull();
        expect(AgentFeedbackConverter.toBarView(asking, some, false)?.action).toBe(
            AgentFeedbackAction.Send
        );
    });

    it('shows a pull request review only when there is something to send', () => {
        const prOpen = Fixtures.agentRun({ phase: AgentPhase.PrOpen });
        expect(AgentFeedbackConverter.toBarView(prOpen, none, false)).toBeNull();
        expect(AgentFeedbackConverter.toBarView(prOpen, some, false)?.textKey).toBe(
            'AGENT.FEEDBACK_BAR.PR_REVIEW'
        );
    });

    it('hides a pull request review while a person holds the task', () => {
        const reviewedByPerson = Fixtures.agentRun({
            phase: AgentPhase.PrOpen,
            isParked: false,
            isAgentAssigned: false
        });
        expect(AgentFeedbackConverter.toBarView(reviewedByPerson, some, false)).toBeNull();
    });

    it('hides for a working, finished or parked run', () => {
        for (const phase of [
            AgentPhase.InProgress,
            AgentPhase.Queued,
            AgentPhase.Done,
            AgentPhase.Failed
        ]) {
            expect(
                AgentFeedbackConverter.toBarView(Fixtures.agentRun({ phase }), some, false)
            ).toBeNull();
        }
        expect(
            AgentFeedbackConverter.toBarView({ ...waitingPlan, isParked: true }, some, false)
        ).toBeNull();
        expect(AgentFeedbackConverter.toBarView(null, some, false)).toBeNull();
    });
});
