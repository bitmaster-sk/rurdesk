import { IssueHistoryField } from '../constants/issue-history-field.enum';

export type IssueHistorySource = 'user' | 'system';

export interface IssueHistory {
    idIssueHistory: number;
    idChange: string;
    field: IssueHistoryField;
    oldValue: unknown;
    newValue: unknown;
    idUser: number | null;
    source: IssueHistorySource;
    createAt: string;
}

export interface IssueHistoryUser {
    idUser: number;
    name: string;
    colorAvatarBg: string;
    avatarInitials: string | null;
    isActive: boolean;
}

export interface IssueHistoryRes {
    history: IssueHistory[];
    users: IssueHistoryUser[];
}
