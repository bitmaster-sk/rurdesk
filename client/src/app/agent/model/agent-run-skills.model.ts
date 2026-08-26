import { AgentStage } from './agent-stage.enum';

export interface AgentRunStageSkills {
    name: AgentStage;
    idsSkill: number[];
    dispatched: boolean;
}

export interface UpdateAgentRunStageSkillsReq {
    stage: AgentStage;
    idsSkill: number[];
}
