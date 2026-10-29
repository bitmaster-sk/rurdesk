import { AgentStage } from './agent-stage.enum';

export interface RunKindStage {
    name: AgentStage;
    hasSkills: boolean;
}

export interface RunKind {
    name: string;
    stages: RunKindStage[];
}
