import { Issue } from '../../issue/model/issue.model';

export interface ProjectBuilderRelation {
    ref: string;
    type: string;
}

export interface ProjectBuilderIssue {
    ref: string;
    title: string;
    description: string;
    estimatedMinutes: number;
    idState: number | null;
    idSeverity: number | null;
    hierarchyParentRef: string;
    scheduleRelations: ProjectBuilderRelation[];
}

export interface ProjectBuilderGenerateReq {
    description: string;
    idState: number | null;
    idSeverity: number | null;
}

export interface ProjectBuilderGenerateRes {
    issues: ProjectBuilderIssue[];
    summary: string;
}

export interface ProjectBuilderAcceptRes {
    issues: Issue[];
}

export interface StagingSnapshot {
    summary: string;
    issues: ProjectBuilderIssue[];
    generatedAt: string;
}
