import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { Router } from '@angular/router';
import { KanbanTile } from '../../entity/kanban-tile.entity';
import { Issue } from '../../../../model/issue.model';
import { IssueSelectionStore } from '../../../../store/issue-selection.store';
import { SelectionClick } from '../../../../util/selection-click';
import { TileSelectionClick } from '../../entity/kanban-selection-click.entity';

@Component({
    selector: 'app-issue-kanban-tile',
    templateUrl: './issue-kanban-tile.component.html',
    styleUrls: ['./issue-kanban-tile.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    // Stable DOM hook for cross-column FLIP (live moves fly between columns)
    host: {
        '[attr.data-tile-id]': 'tile().idIssue',
        '[attr.aria-selected]': 'isSelected()',
        '[attr.tabindex]': 'isHighlighted() ? 0 : -1',
        '(keydown.enter)': 'onEnter($event)'
    }
})
export class KanbanTileComponent {
    private readonly selection = inject(IssueSelectionStore, { optional: true });
    private readonly router = inject(Router);

    public readonly tile = input.required<KanbanTile>();
    public readonly isHighlighted = input(false);
    public readonly contextMenuRequested = output<{ event: MouseEvent; issue: Issue }>();
    public readonly selectionClicked = output<TileSelectionClick>();

    protected readonly isSelected = computed(
        () => !!this.selection?.isSelected(this.tile().idIssuePublic)
    );

    protected onEnter(event: Event): void {
        if (event.target !== event.currentTarget) return;
        const tile = this.tile();
        void this.router.navigate(['/project', tile.idProject, 'issue', tile.idIssuePublic]);
    }

    protected onContextMenu(event: MouseEvent): void {
        event.preventDefault();
        this.contextMenuRequested.emit({ event, issue: this.tile() });
    }

    protected onClick(event: MouseEvent): void {
        const mode = SelectionClick.modeOf(event);
        if (!mode || !this.selection) return;
        SelectionClick.clearTextSelection();
        this.selectionClicked.emit({ mode, issue: this.tile() });
    }
}
