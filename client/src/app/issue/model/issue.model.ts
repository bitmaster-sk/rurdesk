export type CreateIssueReq = Omit<Issue, 'idIssue' | 'idIssuePublic'>;

// A key left out keeps its stored value and an explicit null clears it, so a field
// must never be sent as undefined to mean "empty".
export type UpdateIssueReq = Partial<
    Omit<
        Issue,
        | 'idIssue'
        | 'idIssuePublic'
        | 'idProject'
        | 'createAt'
        | 'updateAt'
        | 'createBy'
        | 'updateBy'
        | 'tracked'
        | 'qualityScore'
        | 'relationCount'
        | 'ganttRank'
        | 'idSprint'
        | 'carryoverCount'
    >
>;

export interface Issue {
    idIssue: number;
    idIssuePublic: number;
    idProject: number;
    idState: number | null;
    idSeverity: number | null;
    idIssueType: number | null;
    title: string;
    description: string;
    createAt?: Date;
    updateAt?: Date;
    createBy?: number;
    updateBy?: number;
    assignedTo?: number | null;
    tracked: number;
    estimated?: number | null;
    scheduledAt?: Date | null;
    qualityScore?: number | null;
    idGitIntegration?: number | null;
    mrId?: string | null;
    relationCount?: number;
    ganttRank?: string | null;
    idSprint?: number | null;
    points?: number | null;
    carryoverCount?: number;
    customFields?: Record<string, string | number | boolean | null>;
}
