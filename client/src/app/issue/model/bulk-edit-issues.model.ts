export interface BulkEditIssueEntry {
    idIssuePublic: number;
    scheduledAt?: string;
    estimated?: number;
}

export interface BulkEditIssues {
    issues: BulkEditIssueEntry[];
}
