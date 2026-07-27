export interface Issue {
    idIssue?: number;
    idIssuePublic?: number;
    idProject: number;
    idState: number | null;
    idSeverity: number | null;
    title: string;
    description: string;
    createAt?: Date;
    updateAt?: Date;
    createBy?: number;
    updateBy?: number;
    assignedTo?: number;
    tracked: number;
    estimated?: number;
    scheduledAt?: Date;
    qualityScore?: number | null;
    idGitIntegration?: number | null;
    mrId?: string | null;
    relationCount?: number;
    ganttRank?: string | null;
    idSprint?: number | null;
    points?: number | null;
    carryoverCount?: number;
}
