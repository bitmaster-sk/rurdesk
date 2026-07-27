import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { IssueState } from '../../../state/model/issue-state.model';
import { IssueSeverity } from '../../../severity/model/issue-severity.model';
import { IssueChangeEvent } from '../project-builder-step-staging/project-builder-step-staging.component';

@Component({
    selector: 'app-staged-issue-tree',
    templateUrl: './staged-issue-tree.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class StagedIssueTreeComponent {
    public roots = input.required<StagedIssueNode[]>();
    public states = input.required<IssueState[]>();
    public severities = input.required<IssueSeverity[]>();

    public issueChange = output<IssueChangeEvent>();
    public deleteNode = output<StagedIssueNode>();
}
