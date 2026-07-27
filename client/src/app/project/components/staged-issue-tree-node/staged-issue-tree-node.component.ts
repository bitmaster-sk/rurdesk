import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { StagedIssueNode } from '../../model/staged-issue-node.model';
import { IssueState } from '../../../state/model/issue-state.model';
import { IssueSeverity } from '../../../severity/model/issue-severity.model';
import { StagedIssue } from '../../../shared/staged-issue/staged-issue.model';
import { IssueChangeEvent } from '../project-builder-step-staging/project-builder-step-staging.component';

@Component({
    selector: 'app-staged-issue-tree-node',
    templateUrl: './staged-issue-tree-node.component.html',
    styleUrls: ['./staged-issue-tree-node.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class StagedIssueTreeNodeComponent {
    public node = input.required<StagedIssueNode>();
    public states = input.required<IssueState[]>();
    public severities = input.required<IssueSeverity[]>();
    public isLastChild = input.required<boolean>();
    public ancestorHasMoreSiblings = input.required<boolean[]>();

    public issueChange = output<IssueChangeEvent>();
    public deleteNode = output<StagedIssueNode>();

    protected depth = computed(() => this.ancestorHasMoreSiblings().length + 1);
    protected stripWidth = computed(() => `${this.depth() * 20}px`);
    protected childAncestorHasMoreSiblings = computed(() => [
        ...this.ancestorHasMoreSiblings(),
        !this.isLastChild()
    ]);

    // The shared card edits only the StagedIssue fields; merge them back onto the
    // full node data so hierarchy/schedule fields are preserved.
    public onCardIssueChange(updated: StagedIssue): void {
        this.issueChange.emit({
            node: this.node(),
            updated: { ...this.node().data, ...updated }
        });
    }

    public onCardDelete(): void {
        this.deleteNode.emit(this.node());
    }
}
