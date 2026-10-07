import { CdkDragDrop, moveItemInArray } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiTreeEntry } from '../../entity/wiki-tree-entry.entity';
import { WikiMoveRequest } from '../../model/wiki-save.model';

export interface WikiTreeMove extends WikiMoveRequest {
    idPage: number;
}

@Component({
    selector: 'app-wiki-tree',
    templateUrl: './wiki-tree.component.html',
    styleUrls: ['./wiki-tree.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiTreeComponent {
    public readonly entries = input.required<WikiTreeEntry[]>();
    public readonly idProject = input.required<number>();
    public readonly spaceKind = input.required<WikiSpaceKind>();
    public readonly activeIdPage = input<number | null>(null);
    public readonly idHomePage = input<number | null>(null);
    public readonly expanded = input<ReadonlySet<number>>(new Set());
    public readonly canEdit = input(false);

    public readonly toggled = output<number>();
    public readonly moved = output<WikiTreeMove>();

    protected readonly AgentAccess = WikiAgentAccess;

    protected link(entry: WikiTreeEntry): (string | number)[] {
        return ['/project', this.idProject(), 'wiki', this.spaceKind(), entry.node.slug];
    }

    protected isOpen(entry: WikiTreeEntry): boolean {
        return this.expanded().has(entry.node.idPage);
    }

    protected onToggle(event: Event, entry: WikiTreeEntry): void {
        event.preventDefault();
        event.stopPropagation();
        this.toggled.emit(entry.node.idPage);
    }

    protected onDrop(event: CdkDragDrop<WikiTreeEntry[]>, idParent: number | null): void {
        if (event.previousIndex === event.currentIndex) {
            return;
        }
        const reordered = [...event.container.data];
        moveItemInArray(reordered, event.previousIndex, event.currentIndex);
        const entry = reordered[event.currentIndex];
        this.moved.emit({
            idPage: entry.node.idPage,
            idParent,
            idPrev: reordered[event.currentIndex - 1]?.node.idPage ?? null,
            idNext: reordered[event.currentIndex + 1]?.node.idPage ?? null
        });
    }
}
