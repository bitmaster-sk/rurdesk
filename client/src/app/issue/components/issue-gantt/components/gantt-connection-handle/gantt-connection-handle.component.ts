import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { HandleSide } from '../../constants/gantt-handle-side.enum';

export { HandleSide };

@Component({
    selector: 'app-gantt-connection-handle',
    templateUrl: './gantt-connection-handle.component.html',
    styleUrls: ['./gantt-connection-handle.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttConnectionHandleComponent {
    protected readonly HandleSide = HandleSide;

    public readonly side = input.required<HandleSide>();
    public readonly taskId = input.required<number>();
    public readonly isOffset = input<boolean>(false);
    /** True while an in-progress relation draw would connect here on release. */
    public readonly isDropActive = input<boolean>(false);

    public readonly dragStarted = output<{
        taskId: number;
        side: HandleSide;
        event: MouseEvent;
    }>();

    public onMouseDown(event: MouseEvent): void {
        event.stopPropagation();
        event.preventDefault();
        this.dragStarted.emit({
            taskId: this.taskId(),
            side: this.side(),
            event
        });
    }
}
