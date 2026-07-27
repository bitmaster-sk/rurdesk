import { Issue } from './issue.model';

export interface IssuesPage {
    items: Issue[];
    nextCursor: string | null;
    total: number;
}

export interface IssueGroup {
    key: Record<string, number | null>;
    items: Issue[];
    total: number;
    nextCursor: string | null;
}
