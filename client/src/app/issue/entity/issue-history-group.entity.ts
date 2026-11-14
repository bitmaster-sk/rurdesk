import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Tag } from 'src/app/tag/model/tag.model';
import { IssueHistory, IssueHistorySource, IssueHistoryUser } from '../model/issue-history.model';

export interface IssueHistoryGroup {
    idChange: string;
    date: Date;
    idUser: number | null;
    source: IssueHistorySource;
    changes: IssueHistory[];
}

export interface IssueHistoryLookups {
    states: Map<number, IssueState>;
    severities: Map<number, IssueSeverity>;
    issueTypes: Map<number, IssueType>;
    users: Map<number, IssueHistoryUser>;
    tags: Map<number, Tag>;
}

export type HistoryValue =
    | { kind: 'empty'; labelKey: string }
    | { kind: 'text'; text: string }
    | { kind: 'translated'; labelKey: string }
    | { kind: 'state'; state: IssueState }
    | { kind: 'severity'; severity: IssueSeverity }
    | { kind: 'issueType'; issueType: IssueType }
    | { kind: 'user'; user: IssueHistoryUser };

export interface HistoryTag {
    idTag: number;
    name: string;
    color: string | null;
}

export type HistoryRow =
    | { kind: 'description' }
    | { kind: 'tags'; labelKey: string; added: HistoryTag[]; removed: HistoryTag[] }
    | { kind: 'value'; labelKey: string; oldValue: HistoryValue; newValue: HistoryValue };
