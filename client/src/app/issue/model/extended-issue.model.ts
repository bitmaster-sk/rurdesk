import { Issue } from './issue.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { User } from 'src/app/auth/model/user.model';

export interface ExtendedIssue extends Issue {
    state: IssueState | undefined;
    severity: IssueSeverity | undefined;
    assignedToUser: User | undefined;
}
