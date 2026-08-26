import { AgentStage } from 'src/app/agent/model/agent-stage.enum';

export interface ProjectSkill {
    idProject: number;
    idSkill: number;
    stage: AgentStage;
}

export interface UpdateProjectSkillReq {
    idSkill: number;
    stage: AgentStage;
}
