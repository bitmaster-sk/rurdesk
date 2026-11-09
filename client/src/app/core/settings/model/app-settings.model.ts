import { AgentEffort } from 'src/app/agent/model/agent-effort.enum';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentStageOptionsByStage } from 'src/app/agent/model/agent-stage-options.model';

export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
    sprintVelocityLimit: number;
    userApiKeyLimit: number;
    isAgentThinkingPersisted: boolean;
    agentThinkingMaxKb: number;
    attachmentMaxSizeMb: number;
    agentStageDefaults: AgentStageOptionsByStage;
    wikiAlwaysTokenLimit: number;
    plantUmlServerUrl: string;
}

export interface AgentStageOptionsChange {
    maxTurns?: number;
    effort?: AgentEffort | '';
}

export type UpdateAppSettingsReq = Partial<Omit<AppSettings, 'agentStageDefaults'>> & {
    agentStageDefaults?: Partial<Record<AgentStage, AgentStageOptionsChange>>;
};
