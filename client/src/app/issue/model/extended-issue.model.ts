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

export type Scheduled<T extends Issue> = T & { scheduledAt: Date };
export type ScheduledIssue = Scheduled<ExtendedIssue>;

/**
 * Type guard that rejects `Invalid Date` — `instanceof Date` alone is not
 * enough because `new Date('nonsense')` is still a Date instance but has
 * `getTime() === NaN`. Widened to `{ scheduledAt?: Date | null }` so the
 * same guard serves both `Issue`-based and structural-type call sites.
 */
export abstract class IssueGuard {
    public static isScheduled<T extends { scheduledAt?: Date | null }>(
        issue: T
    ): issue is T & { scheduledAt: Date } {
        return issue.scheduledAt instanceof Date && !isNaN(issue.scheduledAt.getTime());
    }
}
