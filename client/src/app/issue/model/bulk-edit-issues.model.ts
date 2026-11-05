export interface BulkEditIssueChangesReq {
    scheduledAt?: string;
    estimated?: number;
    idState?: number;
    idSeverity?: number;
    idIssueType?: number;
    idUserAssigned?: number | null;
    addIdsTag?: number[];
    removeIdsTag?: number[];
}

export interface BulkEditIssueEntryReq extends BulkEditIssueChangesReq {
    idIssuePublic: number;
}

export interface BulkEditIssuesReq {
    issues: BulkEditIssueEntryReq[];
}
