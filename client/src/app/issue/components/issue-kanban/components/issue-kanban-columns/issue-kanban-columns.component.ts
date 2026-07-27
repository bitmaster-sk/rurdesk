import {
    ChangeDetectionStrategy,
    Component,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { CdkDragDrop, CdkDragStart } from '@angular/cdk/drag-drop';
import { KanbanColumn } from '../../entity/kanban-column.entity';
import { Issue } from '../../../../model/issue.model';
import { SprintDragService } from '../../service/sprint-drag.service';

@Component({
    selector: 'app-issue-kanban-columns',
    templateUrl: './issue-kanban-columns.component.html',
    styleUrls: ['./issue-kanban-columns.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueKanbanColumnsComponent {
    protected readonly dragSvc = inject(SprintDragService);

    public readonly columns = input.required<KanbanColumn[]>();
    /** Extra drop-list ids (sprint tabs) each column connects to, for drag-to-sprint. */
    public readonly connectedTo = input<string[]>([]);

    public readonly stateChange = output<CdkDragDrop<KanbanColumn>>();
    public readonly contextMenuRequested = output<{ event: MouseEvent; issue: Issue }>();
    public readonly loadMore = output<KanbanColumn>();

    protected readonly gridTemplateColumns = computed(
        () => `repeat(${this.columns().length}, minmax(0, 1fr))`
    );

    // Width of the dragged card, so the preview (a real tile render) matches it
    protected readonly dragPreviewWidth = signal<number | null>(null);

    protected onTileDragStarted(event: CdkDragStart): void {
        this.dragPreviewWidth.set(event.source.element.nativeElement.getBoundingClientRect().width);
    }

    // Tile that just landed after a drop — briefly gets a "settle" animation
    protected readonly landedTileId = signal<number | null>(null);
    private landTimer: ReturnType<typeof setTimeout> | null = null;

    protected onDropped(event: CdkDragDrop<KanbanColumn>): void {
        const tile = event.previousContainer?.data?.tiles?.[event.previousIndex];
        if (tile) {
            this.landedTileId.set(tile.idIssue);
            if (this.landTimer) clearTimeout(this.landTimer);
            this.landTimer = setTimeout(() => this.landedTileId.set(null), 500);
        }
        this.stateChange.emit(event);
    }
}
