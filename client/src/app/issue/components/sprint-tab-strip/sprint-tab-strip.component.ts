import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    ElementRef,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { CdkDragDrop } from '@angular/cdk/drag-drop';
import { SprintDragService } from '../issue-kanban/service/sprint-drag.service';

/** One tab in the sprint strip. `idSprint === null` is the Backlog tab. */
export interface SprintTab {
    idSprint: number | null;
    label: string;
    isCurrent: boolean;
    isClosed: boolean;
    listId: string;
}

@Component({
    selector: 'app-sprint-tab-strip',
    templateUrl: './sprint-tab-strip.component.html',
    styleUrls: ['./sprint-tab-strip.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SprintTabStripComponent implements AfterViewInit {
    protected readonly dragSvc = inject(SprintDragService);

    public readonly tabs = input.required<SprintTab[]>();
    public readonly selectedIdSprint = input<number | null>(null);

    public readonly select = output<number | null>();
    public readonly createRequested = output<void>();
    public readonly editRequested = output<number>();
    public readonly taskDropped = output<{
        idSprint: number | null;
        event: CdkDragDrop<unknown>;
    }>();

    /** Backlog tab is pinned left; cycle tabs scroll in the middle; ＋ is pinned right. */
    protected readonly backlogTab = computed(() => this.tabs().find(t => t.idSprint === null));
    protected readonly cycleTabs = computed(() => this.tabs().filter(t => t.idSprint !== null));

    private readonly scroller = viewChild.required<ElementRef<HTMLElement>>('scroller');
    protected readonly canScrollLeft = signal(false);
    protected readonly canScrollRight = signal(false);

    constructor() {
        // Recompute arrow visibility when the tab set changes (deferred until the
        // new tabs are laid out).
        effect(() => {
            this.tabs();
            setTimeout(() => this.updateScrollState());
        });
    }

    public ngAfterViewInit(): void {
        this.updateScrollState();
    }

    protected onScroll(): void {
        this.updateScrollState();
    }

    protected scrollBy(direction: -1 | 1): void {
        const el = this.scroller().nativeElement;
        el.scrollBy({ left: direction * Math.max(200, el.clientWidth * 0.6), behavior: 'smooth' });
    }

    protected onDrop(event: CdkDragDrop<unknown>, idSprint: number | null): void {
        this.dragSvc.hoveredName.set(null);
        this.taskDropped.emit({ idSprint, event });
    }

    private updateScrollState(): void {
        const el = this.scroller().nativeElement;
        this.canScrollLeft.set(el.scrollLeft > 1);
        this.canScrollRight.set(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
    }
}
