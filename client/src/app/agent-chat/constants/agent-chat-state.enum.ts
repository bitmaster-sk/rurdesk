/** What the conversation header shows about the agent; `Answering` is the owner's own turn. */
export enum AgentChatState {
    Idle = 'idle',
    Answering = 'answering',
    Busy = 'busy',
    Queued = 'queued'
}
