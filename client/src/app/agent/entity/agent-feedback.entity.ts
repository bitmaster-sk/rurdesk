import { AgentFeedbackAction } from '../constants/agent-feedback-action.enum';

/** People's comments the agent has not been given yet. */
export interface AgentFeedback {
    comments: number;
    replies: number;
    threads: number;
}

/** What the feedback bar says and which actions it offers. */
export interface AgentFeedbackBarView {
    textKey: string;
    /** Stage waiting for review, an AGENT.STAGE.* key; null when the text names no stage. */
    stageKey: string | null;
    action: AgentFeedbackAction | null;
    /** Next to Send: approve after a confirmation, the new comments going unsent as feedback. */
    isApproveAnywayOffered: boolean;
}
