import { Issue } from 'src/app/issue/model/issue.model';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';

export interface ReleaseTaskRow {
    issue: Issue;
    issueType: IssueType | null;
    severity: IssueSeverity | null;
    state: IssueState | null;
}
