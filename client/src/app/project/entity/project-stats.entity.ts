import { IssueSeverity } from '../../severity/model/issue-severity.model';
import { IssueState } from '../../state/model/issue-state.model';

export interface ProjectStats {
    totalIssues: number;
    totalEstimatedSeconds: number;
    totalTrackedSeconds: number;
    issuesByState: Map<number, [IssueState, number]>;
    issuesBySeverity: Map<number, [IssueSeverity, number]>;
    openIssuesByAssignee: Map<number | null, number>;
}
