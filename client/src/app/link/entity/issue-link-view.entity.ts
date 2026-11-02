import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueLinkUser } from '../model/issue-link.model';

export interface IssueLinkView {
    idProject: number;
    idIssuePublic: number;
    title: string;
    projectName: string;
    state: IssueState | null;
    isFinal: boolean;
    severity: IssueSeverity | null;
    issueType: IssueType | null;
    assignee: IssueLinkUser | null;
}
