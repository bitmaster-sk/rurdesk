import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    OnDestroy,
    OnInit,
    TemplateRef,
    computed,
    effect,
    inject,
    signal,
    viewChild,
    viewChildren
} from '@angular/core';
import { Router } from '@angular/router';
import { HotkeyService } from 'src/app/core/command/hotkey.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { first, map, switchMap } from 'rxjs/operators';

import { Project } from 'src/app/project/model/project.model';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { SavedViewConfigConverter } from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { IssueRelationRow } from './entity/issue-table-row.entity';
import { IssueTableService } from './service/issue-table.service';
import { UiTableSortEvent } from 'src/app/ui/components/table/table-sort.directive';
import { Issue } from '../../model/issue.model';
import { RelationDropEvent } from './components/issue-table-drop-zone/issue-table-drop-zone.component';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../constants/issue-relation-subtype.enum';
import { IssueQuickActionsComponent } from '../issue-quick-actions/issue-quick-actions.component';
import { resolveHighlightIndex } from './highlight.util';

@Component({
    selector: 'app-issue-table',
    templateUrl: './issue-table.component.html',
    styleUrls: ['./issue-table.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [IssueTableService],
    standalone: false
})
export class IssueTableComponent implements OnInit, AfterViewInit, OnDestroy {
    private readonly toolbarRef = viewChild.required<TemplateRef<unknown>>('toolbar');

    private readonly quickActionsRef =
        viewChild.required<IssueQuickActionsComponent>('quickActions');

    private readonly issueTableService = inject(IssueTableService);
    private readonly projectStore = inject(ProjectStore);
    private readonly stateStore = inject(StateStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly savedViewStore = inject(SavedViewStore);
    private readonly issueToolbarService = inject(IssueToolbarService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly router = inject(Router);
    private readonly hotkeys = inject(HotkeyService);
    private readonly commandPalette = inject(CommandPaletteService);
    private readonly noticeService = inject(NoticeService);
    private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

    /**
     * Keyboard row highlight (list `j`/`k`/↑↓ via HotkeyService), tracked by the
     * task's stable public id — NOT by position. `highlightedIndex` is derived, so
     * when the row list is replaced (edit/refresh/re-order) the highlight follows
     * the same task rather than pointing at whatever now sits at the old index.
     */
    public readonly highlightedId = signal<number | null>(null);

    public readonly highlightedIndex = computed(() =>
        resolveHighlightIndex(this.rows(), this.highlightedId())
    );

    public readonly highlightedIssue = computed<Issue | null>(() => {
        const index = this.highlightedIndex();
        return index === null ? null : (this.rows()[index]?.issue ?? null);
    });

    private readonly rowRefs = viewChildren<ElementRef<HTMLTableRowElement>>('rowRef');

    // Default: newest issue first, by id (stable — editing a field never re-orders
    // the row, unlike updateAt which bumped the edited row to the top).
    protected readonly sortField = signal<string>('idIssue');
    protected readonly sortOrder = signal<1 | -1>(-1);

    public readonly defaultSortColumn = 'idIssue';

    public readonly defaultSortOrder = -1;

    // Keep the command palette's `>` action target in sync with the highlighted
    // task — reacts to both keyboard navigation and the highlighted row's data
    // changing under a refresh (so actions always target current issue data).
    private readonly syncPaletteContext = effect(() => {
        this.commandPalette.setContext({
            idProject: this.idProject ?? null,
            issue: this.highlightedIssue()
        });
    });

    public showFilter = toSignal(this.issueFilterStore.showFilter$, {
        initialValue: false
    });

    // Relation mode preferences — persisted to localStorage
    public isRelationMode = signal(localStorage.getItem('issue-relation-mode') === 'true');

    public isAskLag = signal(localStorage.getItem('issue-relation-ask-lag') !== 'false');

    public colCount = computed(() => (this.isRelationMode() ? 10 : 8));

    public isSplitDialogOpen = signal(false);
    public splitIssue = signal<Issue | null>(null);

    public readonly rows = this.issueTableService.rows;

    public readonly total = this.issueTableService.total;

    public readonly isLoading = this.issueTableService.isLoading;

    // Drag state
    public isDragging = signal(false);

    public draggingFromIssue = signal<Issue | null>(null);

    public idIssueDragOver = signal<number | null>(null);

    // Expanded relation rows keyed by idIssuePublic
    public idsExtendedIssue = signal<Set<number>>(new Set());

    // Rows whose relations have already been lazily fetched (avoids refetching on re-expand).
    private readonly relationsLoaded = new Set<number>();

    // Lag dialog
    public showLagDialog = signal(false);

    public lagMinutes = signal<number | null>(null);

    public idProject: number | null = null;

    private dragLeaveTimer: ReturnType<typeof setTimeout> | null = null;

    // dragenter always fires before dragleave on the source element. When we enter
    // a valid target, we set this so the immediately-following dragleave on the
    // source row doesn't start the clear-timer (the expansion row hasn't rendered yet).
    private suppressNextDragLeave = false;

    private pendingRelation: {
        from: Issue;
        to: Issue;
        relationType: IssueRelationType;
        subType: IssueRelationSubType | null;
    } | null = null;

    public ngAfterViewInit(): void {
        this.issueToolbarService.register(this.toolbarRef());
    }

    public ngOnInit(): void {
        this.setInitialFilter();
        this.onSavedViewResetSignal();
        this.issueFilterStore.actualFilter$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(filter => {
                this.sortField.set(filter.orderColumn);
                this.sortOrder.set(filter.orderDirection === 'asc' ? 1 : -1);
            });
        this.projectStore.project$.pipe(first()).subscribe(p => {
            this.idProject = p.idProject;
        });
        // Wire list-mode j/k/↑↓ (gated + dispatched by HotkeyService) to row highlight.
        this.hotkeys.registerListHandler(delta => this.moveHighlight(delta));
        // Palette actions (set state/severity, assign, clone) push the saved task onto the notice
        // stream — reflect those (and any live change) in the list. The changed row
        // pulses once so the update is visible without a toast.
        this.noticeService.issue$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(notice => {
            this.markPulsed(notice.payload?.idIssuePublic);
            this.issueFilterStore.refresh();
        });
    }

    // Rows that just changed (own edit or live update from a teammate) — they get
    // the shared one-shot pulse. Keyed by idIssuePublic; entries expire on a timer
    // (the window covers the async refresh, so the freshly rendered row still pulses).
    public readonly pulsedIds = signal<Set<number>>(new Set());
    private readonly pulseTimers = new Map<number, ReturnType<typeof setTimeout>>();

    private markPulsed(idIssuePublic: number | null | undefined): void {
        if (idIssuePublic == null) return;
        const alreadyPulsing = this.pulsedIds().has(idIssuePublic);
        this.pulsedIds.update(s => new Set(s).add(idIssuePublic));
        // The row is tracked by id, so a repeated update keeps the same element
        // and the `row--pulsed` class stays on — the CSS animation, already
        // finished, won't replay on its own. Restart it so rapid successive
        // updates each pulse instead of showing nothing until the window lapses.
        if (alreadyPulsing) this.restartRowPulse(idIssuePublic);

        const previous = this.pulseTimers.get(idIssuePublic);
        if (previous) clearTimeout(previous);
        this.pulseTimers.set(
            idIssuePublic,
            setTimeout(() => {
                this.pulsedIds.update(s => {
                    const next = new Set(s);
                    next.delete(idIssuePublic);
                    return next;
                });
                this.pulseTimers.delete(idIssuePublic);
            }, 2500)
        );
    }

    private restartRowPulse(idIssuePublic: number): void {
        const el = this.host.nativeElement.querySelector<HTMLElement>(
            `tbody tr[data-flip-id="${idIssuePublic}"]`
        );
        if (!el) return;
        // Clear → reflow → restore: the browser treats the restored animation as
        // a fresh run and replays it from the start. Reverting to '' falls back to
        // the class's animation (honouring prefers-reduced-motion, which sets none).
        el.style.animation = 'none';
        el.getBoundingClientRect();
        el.style.animation = '';
    }

    public ngOnDestroy(): void {
        this.pulseTimers.forEach(timer => clearTimeout(timer));
        this.pulseTimers.clear();
        this.issueToolbarService.clear();
        this.hotkeys.registerListHandler(null);
        // Stop palette `>` actions targeting a row that's no longer on screen.
        this.commandPalette.setContext({ idProject: this.idProject ?? null, issue: null });
    }

    private moveHighlight(delta: 1 | -1): void {
        const rows = this.rows();
        const count = rows.length;
        if (count === 0) return;
        const current = this.highlightedIndex();
        const next =
            current === null
                ? delta > 0
                    ? 0
                    : count - 1
                : Math.min(count - 1, Math.max(0, current + delta));
        // Track by stable id — the palette context follows via `syncPaletteContext`.
        this.highlightedId.set(rows[next]?.issue.idIssuePublic ?? null);
        // Focus the row so the browser scrolls it into view; Enter on it opens the issue.
        this.rowRefs()[next]?.nativeElement.focus();
    }

    public onOpenHighlighted(issue: Issue): void {
        void this.router.navigate(['/project', issue.idProject, 'issue', issue.idIssuePublic]);
    }

    public onLazyLoad(evt: UiTableSortEvent): void {
        this.issueFilterStore.setOrder({
            orderColumn: evt.sortField,
            orderDirection: evt.sortOrder > 0 ? 'asc' : 'desc'
        });
    }

    // --- Context menu ---

    public onContextMenu(event: MouseEvent, issue: Issue): void {
        event.preventDefault();
        this.quickActionsRef().show(event, issue);
    }

    // --- Filter ---

    public onToggleFilter(): void {
        this.issueFilterStore.toggleShowFilter();
    }

    // --- Relation mode ---

    public onRelationModeChange(value: boolean): void {
        this.isRelationMode.set(value);
        localStorage.setItem('issue-relation-mode', String(value));
        if (!value) this.idsExtendedIssue.set(new Set());
    }

    public onAskLagChange(value: boolean): void {
        this.isAskLag.set(value);
        localStorage.setItem('issue-relation-ask-lag', String(value));
    }

    // --- Row expansion ---

    public onToggleRelations(idIssuePublic: number): void {
        const willExpand = !this.idsExtendedIssue().has(idIssuePublic);
        this.idsExtendedIssue.update(s => {
            const next = new Set(s);
            if (next.has(idIssuePublic)) next.delete(idIssuePublic);
            else next.add(idIssuePublic);
            return next;
        });
        if (willExpand && this.idProject !== null && !this.relationsLoaded.has(idIssuePublic)) {
            this.relationsLoaded.add(idIssuePublic);
            this.issueTableService.loadRelationsFor(this.idProject, idIssuePublic);
        }
    }

    public canLoadMore(): boolean {
        return this.issueTableService.canLoadMore();
    }

    public onLoadMore(): void {
        this.issueTableService.loadMore();
    }

    // --- Drag ---

    public onDragStart(event: DragEvent, issue: Issue): void {
        this.isDragging.set(true);
        this.draggingFromIssue.set(issue);
        event.dataTransfer?.setData('text/plain', String(issue.idIssuePublic));
    }

    public onDragEnter(event: DragEvent, issue: Issue): void {
        if (!this.isDragging() || !this.draggingFromIssue()) return;
        if (issue.idIssuePublic === this.draggingFromIssue()!.idIssuePublic) return;
        event.preventDefault();
        this.clearDragLeaveTimer();
        this.suppressNextDragLeave = true;
        this.idIssueDragOver.set(issue.idIssuePublic);
    }

    public onDragLeave(event: DragEvent): void {
        if (this.suppressNextDragLeave) {
            this.suppressNextDragLeave = false;
            return;
        }
        // If cursor moves into the drop-zone row the relatedTarget check
        // handles the target-body-TR → drop-zone-TR transition.
        const related = event.relatedTarget as Element | null;
        if (related?.closest('.drop-zone-row')) {
            return;
        }
        this.dragLeaveTimer = setTimeout(() => {
            this.idIssueDragOver.set(null);
            this.dragLeaveTimer = null;
        }, 80);
    }

    public onDragEnd(): void {
        this.isDragging.set(false);
        this.draggingFromIssue.set(null);
        this.idIssueDragOver.set(null);
        this.suppressNextDragLeave = false;
        this.clearDragLeaveTimer();
    }

    public clearDragLeaveTimer(): void {
        if (this.dragLeaveTimer !== null) {
            clearTimeout(this.dragLeaveTimer);
            this.dragLeaveTimer = null;
        }
    }

    public onDropZone(evt: RelationDropEvent): void {
        const fromIssue = this.draggingFromIssue();
        if (!fromIssue) return;
        this.onDragEnd();

        if (evt.relationType === IssueRelationType.Schedule && this.isAskLag()) {
            this.pendingRelation = {
                from: fromIssue,
                to: evt.toIssue,
                relationType: evt.relationType,
                subType: evt.subType
            };
            this.showLagDialog.set(true);
        } else {
            this.submitRelation(fromIssue, evt.toIssue, evt.relationType, evt.subType, null);
        }
    }

    // --- Lag dialog ---

    public onConfirmLag(): void {
        if (!this.pendingRelation) return;
        const { from, to, relationType, subType } = this.pendingRelation;
        this.submitRelation(from, to, relationType, subType, this.lagMinutes());
        this.closeLagDialog();
    }

    public onCancelLag(): void {
        this.closeLagDialog();
    }

    // --- Delete relation ---

    public onDeleteRelation(issue: Issue, rel: IssueRelationRow): void {
        const idProject = this.idProject;
        if (idProject === null) {
            return;
        }
        this.issueTableService
            .deleteRelation$(idProject, issue.idIssuePublic, rel.idIssueRelation)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe();
    }

    // --- Split ---

    public onSplitRequested(issue: Issue): void {
        this.splitIssue.set(issue);
        this.isSplitDialogOpen.set(true);
    }

    public onSplitAccepted(_children: Issue[]): void {
        this.isSplitDialogOpen.set(false);
    }

    public onSplitCancelled(): void {
        this.isSplitDialogOpen.set(false);
    }

    private onSavedViewResetSignal(): void {
        this.savedViewStore.filterResetSignal$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.setInitialFilter());
    }

    private setInitialFilter(): void {
        this.projectStore.project$
            .pipe(
                first(),
                switchMap(project =>
                    this.stateStore.statesByProject$(project.idProject).pipe(
                        first(),
                        map(states => [project, states] as [Project, IssueState[]])
                    )
                )
            )
            .subscribe(([project, states]: [Project, IssueState[]]) => {
                // A staged view REPLACES the defaults: a field it omits is unfiltered.
                const pending = this.savedViewStore.consumePending(project.idProject);
                if (pending) {
                    this.issueFilterStore.setInitialFilter({
                        ...SavedViewConfigConverter.toFilter(pending.config),
                        idProject: project.idProject
                    });
                    return;
                }
                this.issueFilterStore.setInitialFilter({
                    idProject: project.idProject,
                    stateUnset: true,
                    idsState: states.filter(s => !s.final).map(s => s.idState),
                    idsSeverity: [],
                    severityUnset: true,
                    idsIssueType: [],
                    issueTypeUnset: true,
                    assignedToUnset: true,
                    idsAssignedTo: [],
                    orderColumn: this.defaultSortColumn,
                    orderDirection: this.defaultSortOrder > 0 ? 'asc' : 'desc',
                    createAtFrom: null,
                    createAtTo: null,
                    updateAtFrom: null,
                    updateAtTo: null
                });
            });
    }

    private submitRelation(
        from: Issue,
        to: Issue,
        relationType: IssueRelationType,
        subType: IssueRelationSubType | null,
        lagMinutes: number | null
    ): void {
        const idProject = this.idProject;
        if (idProject === null) {
            return;
        }
        this.issueTableService
            .insertRelation$(idProject, from, to, relationType, subType, lagMinutes)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({ error: () => {} });
    }

    private closeLagDialog(): void {
        this.showLagDialog.set(false);
        this.lagMinutes.set(null);
        this.pendingRelation = null;
    }
}
