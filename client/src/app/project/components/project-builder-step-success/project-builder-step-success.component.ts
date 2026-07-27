import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
    selector: 'app-project-builder-step-success',
    templateUrl: './project-builder-step-success.component.html',
    styleUrls: ['./project-builder-step-success.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ProjectBuilderStepSuccessComponent {
    public createdCount = input.required<number>();

    public goToIssues = output<void>();

    public onGoToIssues(): void {
        this.goToIssues.emit();
    }
}
