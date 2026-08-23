export interface DeleteMigrationOption {
    id: number;
    label: string;
    color?: string;
}

export interface DeleteMigrationUsageItem {
    key: string;
    params?: Record<string, unknown>;
}
