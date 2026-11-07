export interface CascadeResult {
    affectedTasks: Map<number, { scheduledAt: Date }>;
    shiftedSummaries?: Map<number, number>;
}

export interface TaskSnapshot {
    idIssuePublic: number;
    scheduledAt: Date;
    estimated: number;
}
