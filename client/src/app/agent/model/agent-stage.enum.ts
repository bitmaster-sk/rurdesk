export enum AgentStage {
    Pickup = 'pickup',
    Brainstorming = 'brainstorming',
    Design = 'design',
    ImplementationPlan = 'implementation_plan',
    Implementation = 'implementation',
    ReleaseNotes = 'release_notes',
    Chat = 'chat'
}

export const STAGE_LABELS: Record<AgentStage, string> = {
    [AgentStage.Pickup]: 'AGENT.STAGE.PICKUP',
    [AgentStage.Brainstorming]: 'AGENT.STAGE.BRAINSTORMING',
    [AgentStage.Design]: 'AGENT.STAGE.DESIGN',
    [AgentStage.ImplementationPlan]: 'AGENT.STAGE.IMPLEMENTATION_PLAN',
    [AgentStage.Implementation]: 'AGENT.STAGE.IMPLEMENTATION',
    [AgentStage.ReleaseNotes]: 'AGENT.STAGE.RELEASE_NOTES',
    [AgentStage.Chat]: 'AGENT.STAGE.CHAT'
};
