export interface AppSettings {
    tablePageSize: number;
    kanbanPageSize: number;
    ganttBacklogPageSize: number;
}

export type UpdateAppSettingsReq = Partial<AppSettings>;
