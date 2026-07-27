import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { KanbanTile } from '../../entity/kanban-tile.entity';
import { Issue } from '../../../../model/issue.model';

@Component({
    selector: 'app-issue-kanban-tile',
    templateUrl: './issue-kanban-tile.component.html',
    styleUrls: ['./issue-kanban-tile.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    // Stable DOM hook for cross-column FLIP (live moves fly between columns)
    host: { '[attr.data-tile-id]': 'tile().idIssue' }
})
export class KanbanTileComponent {
    public readonly tile = input.required<KanbanTile>();
    public readonly contextMenuRequested = output<{ event: MouseEvent; issue: Issue }>();

    protected onContextMenu(event: MouseEvent): void {
        event.preventDefault();
        this.contextMenuRequested.emit({ event, issue: this.tile() });
    }
}
