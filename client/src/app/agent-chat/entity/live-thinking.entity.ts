import { AgentThinkingEvent } from 'src/app/agent/model/agent-thinking.model';

/** What streamed so far for one turn while the agent answers. */
export interface LiveThinking {
    idTask: number;
    events: AgentThinkingEvent[];
    /** Some of the turn did not reach the page, so the events are not all of it. */
    hasGap: boolean;
}
