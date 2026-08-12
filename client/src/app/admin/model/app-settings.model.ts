export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
    sprintVelocityLimit: number;
}

export type UpdateAppSettingsReq = Partial<AppSettings>;
