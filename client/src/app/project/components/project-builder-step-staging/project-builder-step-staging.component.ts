import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { IssueState } from '../../../state/model/issue-state.model';
import { IssueSeverity } from '../../../severity/model/issue-severity.model';
import { ProjectBuilderIssue } from '../../model/project-builder.model';
import { StagedIssueNode } from '../../model/staged-issue-node.model';

export interface IssueChangeEvent {
    node: StagedIssueNode;
    updated: ProjectBuilderIssue;
}

@Component({
    selector: 'app-project-builder-step-staging',
    templateUrl: './project-builder-step-staging.component.html',
    styleUrls: ['./project-builder-step-staging.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ProjectBuilderStepStagingComponent {
    public stagedIssues = input.required<StagedIssueNode[]>();

    public summary = input.required<string>();

    public states = input.required<IssueState[]>();

    public severities = input.required<IssueSeverity[]>();

    public isAccepting = input.required<boolean>();

    public isRestoreBannerVisible = input.required<boolean>();

    public restoredFrom = input.required<string>();

    public flatCount = input.required<number>();

    public accept = output<void>();

    public back = output<void>();

    public regenerate = output<void>();

    public issueChange = output<IssueChangeEvent>();

    public deleteNode = output<StagedIssueNode>();

    public dismissBanner = output<void>();

    public discardStaging = output<void>();

    public onAccept(): void {
        this.accept.emit();
    }

    public onBack(): void {
        this.back.emit();
    }

    public onRegenerate(): void {
        this.regenerate.emit();
    }

    public onIssueChange(event: IssueChangeEvent): void {
        this.issueChange.emit(event);
    }

    public onDismissBanner(): void {
        this.dismissBanner.emit();
    }

    public onDiscardStaging(): void {
        this.discardStaging.emit();
    }
}
