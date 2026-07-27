import { Issue } from './issue.model';

export interface ProposedIssue {
    title: string;
    description: string;
    idSeverity: number | null;
    idState: number | null;
    estimatedMinutes?: number;
}

export interface SplitPreviewRes {
    children: ProposedIssue[];
}

export interface SplitAcceptRes {
    children: Issue[];
}
