import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { FormControl } from '@angular/forms';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseDraft } from '../../model/release.model';

@Component({
    selector: 'app-release-tasks-step',
    templateUrl: './release-tasks-step.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseTasksStepComponent {
    public readonly idProject = input.required<number>();
    public readonly draft = input<ReleaseDraft | null>(null);
    public readonly tagName = input('');
    public readonly name = input('');
    public readonly isRecreateTag = input(true);
    public readonly openIssues = input.required<FormControl<ReleaseOpenIssues>>();

    protected readonly ReleaseOpenIssues = ReleaseOpenIssues;
    protected readonly closedCount = computed(
        () => this.draft()?.sections.reduce((sum, section) => sum + section.issues.length, 0) ?? 0
    );
}
