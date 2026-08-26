import { AgentStage } from '../model/agent-stage.enum';

// Pickup is absent: no prompt is rendered for it, so a skill attached there
// would never reach an agent.
export const SKILL_STAGES: AgentStage[] = [
    AgentStage.Brainstorming,
    AgentStage.Design,
    AgentStage.ImplementationPlan,
    AgentStage.Implementation
];
