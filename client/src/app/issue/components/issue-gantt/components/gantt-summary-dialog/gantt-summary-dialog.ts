import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { GanttSummaryChoice } from '../../constants/gantt-summary-choice.enum';

@Component({
    selector: 'app-gantt-summary-dialog',
    templateUrl: './gantt-summary-dialog.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttSummaryDialogComponent {
    public readonly title = input.required<string>();
    public readonly estimated = input.required<number>();

    public readonly chosen = output<GanttSummaryChoice>();
    public readonly cancelled = output<void>();

    protected readonly choice = GanttSummaryChoice;
}
