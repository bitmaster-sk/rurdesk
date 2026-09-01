export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
    sprintVelocityLimit: number;
    userApiKeyLimit: number;
    isAgentThinkingPersisted: boolean;
}

export type UpdateAppSettingsReq = Partial<AppSettings>;
