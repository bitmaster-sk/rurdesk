import { User } from 'src/app/auth/model/user.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Issue } from '../../../model/issue.model';
import { IssueRelationRef } from '../../../model/issue-relation.model';

export interface IssueRelationRow {
    idIssueRelation: number;
    labelKey: string;
    ref: IssueRelationRef;
    severity: IssueSeverity | undefined;
    state: IssueState | undefined;
    assigned: User | undefined;
}

export interface IssueTableRow {
    issue: Issue;
    state: IssueState;
    severity: IssueSeverity;
    assigned: User;
    relations: IssueRelationRow[];
}
