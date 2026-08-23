import { Issue } from './issue.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { User } from 'src/app/auth/model/user.model';

export interface ExtendedIssue extends Issue {
    state: IssueState | undefined;
    severity: IssueSeverity | undefined;
    issueType: IssueType | undefined;
    assignedToUser: User | undefined;
}

export type ScheduledIssue = ExtendedIssue & { scheduledAt: Date };
