import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { GanttHeaderRow, GanttColumn } from '../../service/gantt-timeline.service';

@Component({
    selector: 'app-gantt-timeline-header',
    templateUrl: './gantt-timeline-header.html',
    styleUrl: './gantt-timeline-header.scss',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttTimelineHeaderComponent {
    public readonly headerRows = input.required<GanttHeaderRow[]>();
    public readonly columns = input.required<GanttColumn[]>();
    public readonly totalWidth = input.required<number>();
}
