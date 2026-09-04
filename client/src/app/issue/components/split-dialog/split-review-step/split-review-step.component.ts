import { ChangeDetectionStrategy, Component, input, linkedSignal, output } from '@angular/core';
import { v4 as uuidv4 } from 'uuid';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StagedIssue } from 'src/app/shared/staged-issue/staged-issue.model';
import { ProposedIssue } from '../../../model/split.model';

@Component({
    selector: 'app-split-review-step',
    templateUrl: './split-review-step.component.html',
    styleUrls: ['./split-review-step.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SplitReviewStepComponent {
    public initialChildren = input.required<ProposedIssue[]>();
    public isSaving = input.required<boolean>();
    public severities = input<IssueSeverity[]>([]);
    public states = input<IssueState[]>([]);

    public accept = output<ProposedIssue[]>();
    public cancelled = output<void>();

    // Seeded from initialChildren and re-seeded only if that input changes. The
    // orchestrator sets initialChildren exactly once (loading→review), so user
    // edits persist for the whole review session. `ref` is a client-only list
    // key (the API model has none) — never sent back on accept.
    public readonly issues = linkedSignal<ProposedIssue[], StagedIssue[]>({
        source: () => this.initialChildren(),
        computation: children => children.map(child => this.toStaged(child))
    });

    public onIssueChange(updated: StagedIssue): void {
        this.issues.update(list =>
            list.map(issue => (issue.ref === updated.ref ? updated : issue))
        );
    }

    public onAddChild(): void {
        this.issues.update(list => [
            ...list,
            {
                ref: uuidv4(),
                title: '',
                description: '',
                idSeverity: null,
                idState: null,
                estimatedMinutes: 0
            }
        ]);
    }

    public onRemoveChild(ref: string): void {
        this.issues.update(list => list.filter(issue => issue.ref !== ref));
    }

    public onAccept(): void {
        this.accept.emit(this.issues().map(issue => this.toProposed(issue)));
    }

    public onCancel(): void {
        this.cancelled.emit();
    }

    private toStaged(child: ProposedIssue): StagedIssue {
        return {
            ref: uuidv4(),
            title: child.title,
            description: child.description,
            idSeverity: child.idSeverity,
            idState: child.idState,
            estimatedMinutes: child.estimatedMinutes ?? 0
        };
    }

    private toProposed(issue: StagedIssue): ProposedIssue {
        return {
            title: issue.title,
            description: issue.description,
            idSeverity: issue.idSeverity,
            idState: issue.idState,
            estimatedMinutes: issue.estimatedMinutes
        };
    }
}
