export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
    sprintVelocityLimit: number;
    userApiKeyLimit: number;
    isAgentThinkingPersisted: boolean;
    agentThinkingMaxKb: number;
    attachmentMaxSizeMb: number;
}

export type UpdateAppSettingsReq = Partial<AppSettings>;
