import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';

export interface IssueLinkOption {
    idProject: number;
    projectName: string;
    idIssuePublic: number;
    title: string;
}

export interface IssueLinkRef {
    idProject: number;
    idIssuePublic: number;
}

export interface IssueLinkIssue extends IssueLinkRef {
    title: string;
    idState: number | null;
    idSeverity: number | null;
    idIssueType: number | null;
    assignedTo: number | null;
}

export interface IssueLinkProject {
    idProject: number;
    name: string;
}

export interface IssueLinkUser {
    idUser: number;
    name: string;
    colorAvatarBg: string;
    avatarInitials?: string;
}

export interface IssueLinkInfo {
    issues: IssueLinkIssue[];
    projects: IssueLinkProject[];
    states: IssueState[];
    severities: IssueSeverity[];
    issueTypes: IssueType[];
    users: IssueLinkUser[];
}
