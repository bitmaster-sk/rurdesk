import { AgentEffort } from '../model/agent-effort.enum';
import { AgentStage } from '../model/agent-stage.enum';

export enum AgentEffortOrigin {
    Run = 'run',
    Agent = 'agent',
    Settings = 'settings',
    Model = 'model'
}

export interface AgentEffortInherited {
    effort: AgentEffort | null;
    origin: AgentEffortOrigin;
    agentName: string | null;
}

export interface AgentStageEffortInherited {
    stage: AgentStage;
    inherited: AgentEffortInherited;
}
