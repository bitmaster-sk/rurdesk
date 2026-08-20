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
import { SprintDragService } from '../../service/sprint-drag.service';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { Issue } from '../../../../model/issue.model';
import { SwimlaneCell } from '../../entity/swimlane-cell.entity';
import { SwimlaneRow } from '../../entity/swimlane-row.entity';

@Component({
    selector: 'app-issue-kanban-swimlane',
    templateUrl: './issue-kanban-swimlane.component.html',
    styleUrls: ['./issue-kanban-swimlane.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueKanbanSwimlaneComponent {
    protected readonly dragSvc = inject(SprintDragService);

    public readonly rows = input.required<SwimlaneRow[]>();
    public readonly states = input.required<IssueState[]>();
    /** Extra drop-list ids (sprint tabs) each cell connects to, for drag-to-sprint. */
    public readonly connectedTo = input<string[]>([]);

    public readonly cardDrop = output<CdkDragDrop<SwimlaneCell>>();
    public readonly contextMenuRequested = output<{ event: MouseEvent; issue: Issue }>();
    public readonly cellLoadMore = output<SwimlaneCell>();

    protected readonly sortedRows = computed(() =>
        [...this.rows()].sort((a, b) => {
            if (!a.user) return 1;
            if (!b.user) return -1;
            return a.user.name.localeCompare(b.user.name);
        })
    );

    protected readonly gridTemplateColumns = computed(
        () => `150px repeat(${this.states().length}, minmax(0, 1fr))`
    );

    // Plain method (not computed): drops mutate cell.total in place, so this must
    // re-evaluate on every change-detection pass, not only when rows() changes.
    protected stateTotal(idState: number): number {
        return this.rows().reduce(
            (sum, row) => sum + (row.cells.find(c => c.state.idState === idState)?.total ?? 0),
            0
        );
    }

    // Width of the dragged card, so the preview (a real tile render) matches it
    protected readonly dragPreviewWidth = signal<number | null>(null);

    protected onTileDragStarted(event: CdkDragStart): void {
        this.dragPreviewWidth.set(event.source.element.nativeElement.getBoundingClientRect().width);
    }

    // Tile that just landed after a drop — briefly gets a "settle" animation
    protected readonly landedTileId = signal<number | null>(null);
    private landTimer: ReturnType<typeof setTimeout> | null = null;

    protected onDropped(event: CdkDragDrop<SwimlaneCell>): void {
        const tile = event.previousContainer.data.tiles[event.previousIndex];
        if (tile) {
            this.landedTileId.set(tile.idIssue);
            if (this.landTimer) clearTimeout(this.landTimer);
            this.landTimer = setTimeout(() => this.landedTileId.set(null), 500);
        }
        this.cardDrop.emit(event);
    }
}
