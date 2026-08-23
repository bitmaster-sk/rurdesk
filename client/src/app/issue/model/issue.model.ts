export type IssueDraft = Omit<Issue, 'idIssue' | 'idIssuePublic'>;

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
}
