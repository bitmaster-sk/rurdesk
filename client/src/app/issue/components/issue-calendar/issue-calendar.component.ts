import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    HostListener,
    NgZone,
    OnDestroy,
    TemplateRef,
    effect,
    inject,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import {
    Calendar,
    EventClickArg,
    EventContentArg,
    EventDropArg,
    EventInput
} from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import interactionPlugin, { EventResizeDoneArg } from '@fullcalendar/interaction';
import timeGridPlugin from '@fullcalendar/timegrid';
import { FullCalendarComponent } from '@fullcalendar/angular';
import { CalendarOptions } from '@fullcalendar/core';
import { IssueService } from '../../issue.service';
import { startOfMonth, endOfMonth, add } from 'date-fns';
import { ProjectStore } from 'src/app/project/project.store';
import { first } from 'rxjs/operators';
import cloneDeep from 'lodash-es/cloneDeep';
import enLocale from '@fullcalendar/core/locales/en-gb';
import { Router } from '@angular/router';
import { Issue } from '../../model/issue.model';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { SavedViewConfigConverter } from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { IssueCalendarService } from './service/issue-calendar.service';
import { IssueQuickActionsComponent } from '../issue-quick-actions/issue-quick-actions.component';
import {
    CALENDAR_CARD_MODE_OPTIONS,
    IssueCardViewType
} from '../../constants/issue-card-view-type.constant';
import { CalendarIssueRenderer } from './components/calendar-issue-renderer/calendar-issue-renderer';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import {
    prefersReducedMotion,
    pulseElement,
    UI_SETTLE_DURATION_MS,
    UI_SETTLE_EASING
} from 'src/app/ui/util/motion';
import { TranslateService } from '@ngx-translate/core';

@Component({
    selector: 'app-issue-calendar',
    templateUrl: './issue-calendar.component.html',
    styleUrls: ['./issue-calendar.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [IssueCalendarService],
    standalone: false
})
export class IssueCalendarComponent implements AfterViewInit, OnDestroy {
    private readonly calendarRef = viewChild.required<FullCalendarComponent>('calendar');
    private readonly toolbarRef = viewChild.required<TemplateRef<unknown>>('toolbar');
    private readonly quickActionsRef = viewChild<IssueQuickActionsComponent>('quickActions');

    private readonly i18n = inject(TranslateService);

    private readonly zone = inject(NgZone);
    private readonly router = inject(Router);
    private readonly sIssue = inject(IssueService);
    private readonly projectStore = inject(ProjectStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly savedViewStore = inject(SavedViewStore);
    private readonly issueCalendarService = inject(IssueCalendarService);
    private readonly issueToolbarService = inject(IssueToolbarService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly commandPalette = inject(CommandPaletteService);
    private readonly noticeService = inject(NoticeService);

    // Keeps Calendar import alive to prevent tree-shaking of the plugin
    private readonly _calendar = Calendar;

    public readonly defaultSortColumn = 'scheduledAt';
    public readonly defaultSortOrder = -1;

    private readonly renderer = new CalendarIssueRenderer();
    private readonly issueMap = new Map<number, Issue>();

    public readonly defaultCalendarOps: CalendarOptions = {
        locales: [enLocale],
        locale: 'en',
        plugins: [dayGridPlugin, interactionPlugin, timeGridPlugin],
        headerToolbar: {
            left: 'prev,today,next',
            center: 'title',
            right: ''
        },
        height: '100%',
        expandRows: true,
        initialView: 'dayGridMonth',
        events: [],
        editable: true,
        // Render every event as a block bar (never FC's "dot" layout for timed
        // events) so all events flow through the same fc-event-main → .cal-event-c
        // path — one box model, no stray dot padding or FC's rgba(0,0,0,.1) hover.
        eventDisplay: 'block',
        slotEventOverlap: false,
        eventResizableFromStart: false,
        eventContent: this.onCalendarEventContent.bind(this),
        eventClick: this.onCalendarEventClick.bind(this),
        eventDrop: this.onCalendarEventDrop.bind(this),
        eventResize: this.onCalendarEventResize.bind(this)
    };

    // Landing-in-progress — the refresh triggered by the drop's own WS echo
    // re-renders the event elements mid-animation, so onEventsChange replays it
    // on the fresh element while the window is open.
    private pendingSettle: { idIssue: number; at: number } | null = null;
    private settleAnimation: Animation | null = null;

    /**
     * A dropped event lands with the same spring squash as a dropped kanban
     * card. (A release-point glide is invisible here: FC's drag mirror already
     * snaps to the grid while dragging, so the drop delta is ~0.) Multi-day
     * events settle by their first rendered segment.
     */
    private settleDroppedEvent(idIssue: number): void {
        if (prefersReducedMotion() || document.hidden) return;
        this.pendingSettle = { idIssue, at: Date.now() };
        this.playSettle();
    }

    private playSettle(): void {
        const settle = this.pendingSettle;
        if (!settle) return;
        requestAnimationFrame(() => {
            const segment = this.calendarRef()
                .getApi()
                .el.querySelector(`[data-issue-id="${settle.idIssue}"]`)
                ?.closest('.fc-event') as HTMLElement | null;
            if (!segment) return;
            const elapsed = Date.now() - settle.at;
            if (elapsed >= UI_SETTLE_DURATION_MS) return;
            // Cancel only our own prior settle — a blanket getAnimations()
            // cancel would also kill the pulse ring running on the element.
            const prior = this.settleAnimation;
            if (prior?.playState === 'running') {
                if ((prior.effect as KeyframeEffect | null)?.target === segment) return;
                prior.cancel();
            }
            const animation = segment.animate(
                [
                    { transform: 'scale(0.94)', offset: 0 },
                    { transform: 'scale(1.03)', offset: 0.6 },
                    { transform: 'scale(1)', offset: 1 }
                ],
                { duration: UI_SETTLE_DURATION_MS, easing: UI_SETTLE_EASING }
            );
            // The WS-echo refresh replaces the element mid-squash — continue
            // from where the dead animation left off instead of restarting.
            animation.currentTime = elapsed;
            this.settleAnimation = animation;
        });
    }

    public readonly showFilter = toSignal(this.issueFilterStore.showFilter$, {
        initialValue: false
    });

    public readonly cardMode = signal<IssueCardViewType>(
        this.validCalendarMode(localStorage.getItem('issue-calendar-card-mode'))
    );

    constructor() {
        effect(() => {
            this.showFilter();
            queueMicrotask(() => this.calendarRef()?.getApi()?.updateSize());
        });
    }

    public readonly cardModeOptions = CALENDAR_CARD_MODE_OPTIONS;

    public currentView = 'dayGridMonth';

    public readonly viewOptions = [
        { label: this.i18n.instant('ISSUE.CALENDAR.DAY'), value: 'timeGridDay' },
        { label: this.i18n.instant('ISSUE.CALENDAR.WEEK'), value: 'timeGridWeek' },
        { label: this.i18n.instant('ISSUE.CALENDAR.MONTH'), value: 'dayGridMonth' }
    ];

    public ngAfterViewInit(): void {
        this.issueToolbarService.register(this.toolbarRef());

        this.issueCalendarService.events$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(this.onEventsChange.bind(this));

        // Live updates (own palette edits and teammates' changes): reload the
        // events and pulse the changed one once the fresh render lands.
        this.noticeService.issue$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(notice => {
            if (notice.payload?.idIssue != null) {
                this.startDropPulse(notice.payload.idIssue);
            }
            this.issueFilterStore.refresh();
        });

        // FC's built-in prev/today/next buttons navigate in their own click
        // handler — a capture-phase listener lets us snapshot the outgoing view
        // first, so the datesSet slide has something to push out.
        this.calendarRef()
            .getApi()
            .el.addEventListener(
                'click',
                (e: MouseEvent) => {
                    const isNavButton = (e.target as HTMLElement).closest(
                        '.fc-prev-button, .fc-next-button, .fc-today-button'
                    );
                    if (isNavButton) this.prepareSlideSnapshot();
                },
                true
            );

        this.calendarRef()
            .getApi()
            .on('datesSet', args => {
                this.slideViewOnRangeChange(args.start);
                this.setFilter(args.start, args.end);
            });

        this.calendarRef()
            .getApi()
            .el.addEventListener(
                'contextmenu',
                (e: MouseEvent) => {
                    e.preventDefault();
                    e.stopPropagation();
                    const fcEvent = (e.target as HTMLElement).closest('.fc-event');
                    if (!fcEvent) return;
                    const el = fcEvent.querySelector('[data-issue-id]');
                    if (!el) return;
                    const issueId = Number(el.getAttribute('data-issue-id'));
                    const issue = this.issueMap.get(issueId);
                    if (!issue) return;
                    this.zone.run(() => this.quickActionsRef()?.show(e, issue));
                },
                true
            );

        this.calendarRef()
            .getApi()
            .el.classList.toggle('fc--compact', this.cardMode() === 'CalendarCompact');

        this.setInitialFilter();
        this.onSavedViewResetSignal();
    }

    public ngOnDestroy(): void {
        this.issueToolbarService.clear();
    }

    public onToggleFilter(): void {
        this.issueFilterStore.toggleShowFilter();
    }

    public onViewModeChange(view: string): void {
        this.currentView = view;
        this.calendarRef().getApi().changeView(view);
    }

    // Same keyboard bindings as the Gantt: t/Home → today, ←/→ → prev/next
    // period, +/- → finer/coarser granularity (month → week → day).
    @HostListener('window:keydown', ['$event'])
    public onKeyDown(event: KeyboardEvent): void {
        if (this.commandPalette.isOverlayOpen()) return;
        const target = event.target as HTMLElement | null;
        if (target?.closest('input, textarea, select, [contenteditable]')) return;
        const api = this.calendarRef().getApi();
        switch (event.key) {
            case 't':
            case 'Home':
                this.prepareSlideSnapshot();
                api.today();
                break;
            case 'ArrowLeft':
                this.prepareSlideSnapshot();
                api.prev();
                break;
            case 'ArrowRight':
                this.prepareSlideSnapshot();
                api.next();
                break;
            case '+':
            case '=':
                this.zoomView(1);
                break;
            case '-':
                this.zoomView(-1);
                break;
        }
    }

    private zoomView(direction: number): void {
        const order = ['dayGridMonth', 'timeGridWeek', 'timeGridDay'];
        const idx = order.indexOf(this.currentView);
        const next = idx + direction;
        if (next < 0 || next >= order.length) return;
        this.onViewModeChange(order[next]);
    }

    public onCardModeChange(mode: IssueCardViewType): void {
        this.cardMode.set(mode);
        localStorage.setItem('issue-calendar-card-mode', mode);
        this.calendarRef()
            .getApi()
            .el.classList.toggle('fc--compact', mode === 'CalendarCompact');
        this.calendarRef().getApi().render();
    }

    // Start of the currently displayed range — drives the slide direction.
    private displayedRangeStart: Date | null = null;

    // Static snapshot of the outgoing view, captured just before a navigation.
    // FC has already re-rendered by the time datesSet fires, so without this
    // there is nothing left to push out.
    private slideSnapshot: HTMLElement | null = null;
    private slideSnapshotTimer: ReturnType<typeof setTimeout> | null = null;

    /**
     * Clones the current view and floats it over the calendar. Called right
     * before prev/next/today (keyboard and toolbar buttons alike); if no
     * navigation follows (e.g. "today" while already on today), the clone is
     * discarded by the timer.
     */
    private prepareSlideSnapshot(): void {
        if (prefersReducedMotion() || document.hidden) return;
        this.discardSlideSnapshot();

        const harness = this.calendarRef()
            .getApi()
            .el.querySelector('.fc-view-harness') as HTMLElement | null;
        if (!harness?.parentElement) return;

        // The clone must live OUTSIDE FullCalendar's managed root — FC's render
        // reconciliation removes foreign children inside `.fc` on navigation.
        // The <full-calendar> host element is Angular-managed and safe.
        const fcRoot = this.calendarRef().getApi().el;
        const host = fcRoot.parentElement as HTMLElement | null;
        if (!host) return;
        const hostRect = host.getBoundingClientRect();
        const harnessRect = harness.getBoundingClientRect();

        const clone = harness.cloneNode(true) as HTMLElement;
        clone.style.position = 'absolute';
        clone.style.top = `${harnessRect.top - hostRect.top}px`;
        clone.style.left = `${harnessRect.left - hostRect.left}px`;
        clone.style.width = `${harnessRect.width}px`;
        clone.style.height = `${harnessRect.height}px`;
        clone.style.margin = '0';
        clone.style.zIndex = '10';
        clone.style.pointerEvents = 'none';
        clone.style.background = 'var(--ui-surface-0)';
        clone.style.overflow = 'hidden';
        host.style.position = 'relative';
        host.appendChild(clone);

        this.slideSnapshot = clone;
        this.slideSnapshotTimer = setTimeout(() => this.discardSlideSnapshot(), 300);
    }

    private discardSlideSnapshot(): void {
        if (this.slideSnapshotTimer) clearTimeout(this.slideSnapshotTimer);
        this.slideSnapshotTimer = null;
        this.slideSnapshot?.remove();
        this.slideSnapshot = null;
    }

    /**
     * Directional push when the visible range changes: the snapshot of the old
     * view slides out while the fresh view slides in from the side the user
     * navigated toward. Runs only when a snapshot exists (prev/next/today) —
     * view-mode switches don't slide.
     */
    private slideViewOnRangeChange(start: Date): void {
        const previous = this.displayedRangeStart;
        this.displayedRangeStart = start;

        const snapshot = this.slideSnapshot;
        if (this.slideSnapshotTimer) clearTimeout(this.slideSnapshotTimer);
        this.slideSnapshotTimer = null;
        this.slideSnapshot = null;

        if (!previous || previous.getTime() === start.getTime() || prefersReducedMotion()) {
            snapshot?.remove();
            return;
        }

        const harness = this.calendarRef()
            .getApi()
            .el.querySelector('.fc-view-harness') as HTMLElement | null;
        if (!harness) {
            snapshot?.remove();
            return;
        }

        const distance = Math.max(64, Math.round(harness.offsetWidth * 0.18));
        const dx = start > previous ? distance : -distance;

        if (snapshot) {
            snapshot
                .animate(
                    [
                        { transform: 'translateX(0)', opacity: 1 },
                        { transform: `translateX(${-dx}px)`, opacity: 0 }
                    ],
                    { duration: UI_SETTLE_DURATION_MS, easing: UI_SETTLE_EASING }
                )
                .finished.catch(() => undefined)
                .then(() => snapshot.remove());
            // Belt and braces: `finished` can stall in a backgrounded tab —
            // never leave the static clone covering the live calendar.
            setTimeout(() => snapshot.remove(), UI_SETTLE_DURATION_MS + 300);
        }
        harness.animate(
            [
                { transform: `translateX(${dx}px)`, opacity: snapshot ? 0.55 : 0.35 },
                { transform: 'translateX(0)', opacity: 1 }
            ],
            { duration: UI_SETTLE_DURATION_MS, easing: UI_SETTLE_EASING }
        );
    }

    private setFilter(start?: Date, end?: Date): void {
        this.issueFilterStore.setFilter({
            scheduledAtFrom: !start ? startOfMonth(new Date()) : start,
            scheduledAtTo: !end ? endOfMonth(new Date()) : end
        });
    }

    private onSavedViewResetSignal(): void {
        this.savedViewStore.filterResetSignal$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.setInitialFilter());
    }

    private setInitialFilter(): void {
        this.projectStore.project$.pipe(first()).subscribe(project => {
            const now = new Date();
            // Never persisted in a view, so both branches need it computed here.
            const scheduledAtFrom = startOfMonth(now);
            const scheduledAtTo = endOfMonth(now);
            const pending = this.savedViewStore.consumePending(project.idProject);
            if (pending) {
                this.issueFilterStore.setInitialFilter({
                    ...SavedViewConfigConverter.toFilter(pending.config),
                    scheduledAtFrom,
                    scheduledAtTo,
                    idProject: project.idProject
                });
                return;
            }
            this.issueFilterStore.setInitialFilter({
                idProject: project.idProject,
                stateUnset: true,
                idsState: [],
                idsSeverity: [],
                severityUnset: true,
                assignedToUnset: true,
                idsAssignedTo: [],
                orderColumn: this.defaultSortColumn,
                orderDirection: this.defaultSortOrder > 0 ? 'asc' : 'desc',
                scheduledAtFrom,
                scheduledAtTo
            });
        });
    }

    private onCalendarEventContent(evt: EventContentArg): { domNodes: HTMLElement[] } {
        return this.renderer.render({ evt, cardMode: this.cardMode() });
    }

    private validCalendarMode(stored: string | null): IssueCardViewType {
        return stored === 'CalendarComfort' || stored === 'CalendarCompact'
            ? stored
            : 'CalendarComfort';
    }

    private onCalendarEventClick(evt: EventClickArg): void {
        const issue = evt.event.extendedProps['issue'] as Issue;
        this.router.navigate(['/project', issue.idProject, 'issue', issue.idIssuePublic]);
    }

    private onCalendarEventResize(evt: EventResizeDoneArg): void {
        const issue = cloneDeep(evt.event.extendedProps['issue'] as Issue);
        if (
            evt.startDelta.years +
                evt.startDelta.months +
                evt.startDelta.days +
                evt.startDelta.milliseconds !==
            0
        ) {
            evt.revert();
            return;
        }
        const deltaSeconds = DurationConverter.durationToSeconds({
            years: evt.endDelta.years,
            months: evt.endDelta.months,
            days: evt.endDelta.days,
            seconds: Math.trunc(evt.endDelta.milliseconds / 1000)
        });
        issue.estimated = issue.estimated + deltaSeconds;
        this.sIssue.updateIssue(issue).subscribe({
            error: err => {
                evt.revert();
                throw err;
            }
        });
    }

    private onCalendarEventDrop(evt: EventDropArg): void {
        const issue = evt.event.extendedProps['issue'] as Issue;
        if (evt.event.allDay && !evt.oldEvent.allDay) {
            issue.estimated = null;
        } else if (!evt.event.allDay && evt.oldEvent.allDay) {
            issue.scheduledAt = evt.event.start;
            issue.estimated = 1 * 60 * 60;
        } else {
            issue.scheduledAt = add(issue.scheduledAt, {
                years: evt.delta.years,
                months: evt.delta.months,
                days: evt.delta.days,
                seconds: Math.trunc(evt.delta.milliseconds / 1000)
            });
        }
        this.sIssue.updateIssue(issue).subscribe({
            error: err => {
                evt.revert();
                throw err;
            }
        });
        this.settleDroppedEvent(issue.idIssue);
        this.startDropPulse(issue.idIssue);
    }

    // Issue whose event should pulse after a drop; kept briefly so the pulse can
    // be re-applied when the API refresh re-renders the event elements.
    private pulseIssueId: number | null = null;
    private pulseTimer: ReturnType<typeof setTimeout> | null = null;

    private startDropPulse(idIssue: number): void {
        this.pulseIssueId = idIssue;
        if (this.pulseTimer) clearTimeout(this.pulseTimer);
        this.pulseTimer = setTimeout(() => (this.pulseIssueId = null), 1200);
        this.pulseEventElements(idIssue);
    }

    /**
     * One-shot highlight of the changed event itself — confirms a reschedule or
     * inline edit without a toast. Animates every rendered segment (multi-day
     * events span several cells in month view).
     */
    private pulseEventElements(idIssue: number): void {
        this.calendarRef()
            .getApi()
            .el.querySelectorAll(`[data-issue-id="${idIssue}"]`)
            .forEach(el => {
                const eventEl = el.closest('.fc-event') as HTMLElement | null;
                if (eventEl) pulseElement(eventEl);
            });
    }

    private onEventsChange(events: EventInput[]): void {
        this.issueMap.clear();
        events.forEach(e => {
            const issue = (e.extendedProps as Record<string, unknown>)?.['issue'] as
                Issue | undefined;
            if (issue?.idIssue != null) this.issueMap.set(issue.idIssue, issue);
        });
        this.calendarRef().getApi().removeAllEventSources();
        this.calendarRef().getApi().addEventSource(events);

        // The refresh after a drop replaces the event elements mid-animation —
        // re-apply the pulse and the drop settle to the freshly rendered ones
        // (double rAF: FC renders after the source swap settles).
        const idPulse = this.pulseIssueId;
        if (this.pendingSettle && Date.now() - this.pendingSettle.at > 900) {
            this.pendingSettle = null;
        }
        if (idPulse !== null || this.pendingSettle) {
            requestAnimationFrame(() =>
                requestAnimationFrame(() => {
                    this.playSettle();
                    if (idPulse !== null) this.pulseEventElements(idPulse);
                })
            );
        }
    }

    protected readonly isSplitDialogOpen = signal(false);

    protected readonly splitIssue = signal<Issue | null>(null);

    protected onSplitRequested(issue: Issue): void {
        this.splitIssue.set(issue);
        this.isSplitDialogOpen.set(true);
    }

    protected onSplitAccepted(): void {
        this.isSplitDialogOpen.set(false);
    }

    protected onSplitCancelled(): void {
        this.isSplitDialogOpen.set(false);
    }
}
