import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    computed,
    inject,
    input,
    OnDestroy,
    output,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Location } from '@angular/common';
import { Router } from '@angular/router';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { combineLatest } from 'rxjs';
import { addDays, startOfDay } from 'date-fns';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { Issue } from '../../model/issue.model';
import { IssueService } from '../../issue.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ClipboardService } from 'src/app/core/clipboard.service';

@Component({
    selector: 'app-issue-quick-actions',
    templateUrl: './issue-quick-actions.component.html',
    styleUrls: ['./issue-quick-actions.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueQuickActionsComponent implements OnDestroy {
    private readonly router = inject(Router);
    private readonly location = inject(Location);
    private readonly cdr = inject(ChangeDetectorRef);
    private readonly projectStore = inject(ProjectStore);
    private readonly stateStore = inject(StateStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly memberStore = inject(ProjectMemberStore);
    private readonly issueService = inject(IssueService);
    private readonly toast = inject(ToastNotificationService);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly clipboard = inject(ClipboardService);

    private readonly popoverRef = viewChild.required<UiPopoverComponent>('popover');

    /** Which step the single popover shows. The calendar (`date`) swaps in place
     *  of the actions rather than opening a nested popover — the flatpickr input
     *  is then created in the already-live popover DOM (attached), avoiding the
     *  orphaned-calendar problem a portal-projected input has. */
    protected readonly view = signal<'actions' | 'date'>('actions');

    public readonly showState = input(true);
    public readonly showSeverity = input(true);
    public readonly showAssignee = input(true);
    public readonly showReschedule = input(true);

    public readonly splitRequested = output<Issue>();

    protected readonly issue = signal<Issue | null>(null);
    protected readonly states = signal<IssueState[]>([]);
    protected readonly severities = signal<IssueSeverity[]>([]);
    protected readonly users = signal<User[]>([]);

    protected readonly currentSeverity = computed(
        () => this.severities().find(s => s.idSeverity === this.issue()?.idSeverity) ?? null
    );

    protected readonly currentUser = computed(
        () => this.users().find(u => u.idUser === this.issue()?.assignedTo) ?? null
    );

    protected readonly trackedPercent = computed(() => {
        const issue = this.issue();
        if (!issue?.estimated) return 0;
        return Math.min(100, Math.round((issue.tracked / issue.estimated) * 100));
    });

    private anchor: HTMLElement | null = null;

    public constructor() {
        combineLatest([
            this.projectStore.project$,
            this.stateStore.states$,
            this.severityStore.severities$,
            this.memberStore.users$
        ])
            .pipe(takeUntilDestroyed())
            .subscribe(([project, states, severities, users]) => {
                this.states.set(states.filter(s => s.idProject === project.idProject));
                this.severities.set(severities.filter(s => s.idProject === project.idProject));
                this.users.set(users);
            });
    }

    public ngOnDestroy(): void {
        this.anchor?.remove();
        this.anchor = null;
    }

    public show(event: MouseEvent, issue: Issue): void {
        this.issue.set({ ...issue });
        this.view.set('actions');
        this.cdr.detectChanges();

        this.popoverRef().hide();

        this.anchor?.remove();
        this.anchor = document.createElement('span');
        this.anchor.style.cssText = `position:fixed;top:${event.clientY}px;left:${event.clientX}px;width:1px;height:1px;pointer-events:none;`;
        document.body.appendChild(this.anchor);

        const anchor = this.anchor;
        // Exclude the right-clicked element: the opening right-click's trailing
        // auxclick/pointerup lands on it, and without this the popover would treat
        // that release as an outside click and close on button-up. Use `target`
        // (not `currentTarget`, which is already null here when the event arrived
        // via an @Output / FullCalendar, e.g. kanban & gantt).
        const originEl = event.target as HTMLElement | null;
        setTimeout(() => this.popoverRef().show(anchor, originEl));
    }

    public hide(): void {
        this.popoverRef().hide();
    }

    /** Reset to the actions step whenever the popover closes. */
    protected onPopoverHide(): void {
        this.view.set('actions');
    }

    /** Swap the popover to the calendar step, then re-anchor for its new size. */
    protected onOpenDateView(): void {
        this.view.set('date');
        this.repositionSoon();
    }

    /** Back to the actions step (cancel), re-anchoring for its size. */
    protected onCloseDateView(): void {
        this.view.set('actions');
        this.repositionSoon();
    }

    private repositionSoon(): void {
        // The view swap re-renders on the next CD pass; reposition after that so
        // the panel re-anchors to the new content height instead of the old.
        setTimeout(() => this.popoverRef().reposition());
    }

    /** flatpickr's calendar is lazily imported, so it appears after the view swap.
     *  Re-anchor once it exists, else it overflows the pre-calendar-sized popover
     *  (its right-edge nav arrow gets clipped). */
    protected onCalendarReady(): void {
        this.popoverRef().reposition();
    }

    protected buildInitials(name: string): string {
        const parts = name.trim().split(' ');
        if (parts.length > 1) {
            return parts
                .slice(0, 2)
                .map(p => p[0])
                .join('')
                .toUpperCase();
        }
        return parts[0].substring(0, 2).toUpperCase();
    }

    protected onStateChange(idState: number): void {
        this.patch({ idState });
    }

    protected onSeverityChange(idSeverity: number): void {
        this.patch({ idSeverity });
    }

    protected onAssigneeChange(userId: number | null): void {
        this.patch({ assignedTo: userId });
    }

    protected onPreviousDay(): void {
        const scheduledAt = this.issue()?.scheduledAt;
        if (!scheduledAt) return;
        this.patch({ scheduledAt: addDays(scheduledAt, -1) });
    }

    protected onToday(): void {
        this.patch({ scheduledAt: startOfDay(new Date()) });
    }

    protected onNextDay(): void {
        const scheduledAt = this.issue()?.scheduledAt;
        if (!scheduledAt) return;
        this.patch({ scheduledAt: addDays(scheduledAt, 1) });
    }

    protected onRemoveDate(): void {
        this.patch({ scheduledAt: null });
    }

    protected onPickDate(date: Date): void {
        if (!date) return;
        this.patch({ scheduledAt: startOfDay(date) });
        this.view.set('actions');
        this.repositionSoon();
    }

    // One optimistic write for every quick action. Without the rollback the
    // popover shows the new value while the board behind it keeps the old one,
    // and nothing tells the user the save failed.
    private patch(over: Partial<Issue>): void {
        const previous = this.issue();
        if (!previous) return;
        this.issue.set({ ...previous, ...over });
        this.issueService.updateIssue({ ...previous, ...over }).subscribe({
            next: () => this.issueFilterStore.refresh(),
            error: () => {
                this.issue.set(previous);
                this.issueFilterStore.refresh();
            }
        });
    }

    protected onOpen(): void {
        const issue = this.issue();
        if (!issue) return;

        const urlTree = this.router.createUrlTree(['/project', issue.idProject, 'issue', issue.idIssuePublic]);
        const internalUrl = this.router.serializeUrl(urlTree);
        const externalUrl = this.location.prepareExternalUrl(internalUrl);
        const fullUrl = `${window.location.origin}${externalUrl}`;

        window.open(fullUrl, '_blank', 'noopener');
        this.hide();
    }

    protected onCopyId(): void {
        const issue = this.issue();
        if (!issue) return;
        // Clipboard writes reject when the document isn't focused or permission is
        // denied — it's best-effort UX, so swallow the failure instead of leaving an
        // unhandled rejection.
        void this.clipboard.copy(String(issue.idIssuePublic));
        this.hide();
    }

    protected onDelete(): void {
        const issue = this.issue();
        if (!issue) return;
        this.issueService.deleteIssue(issue.idProject, issue.idIssuePublic).subscribe(() => {
            this.issueFilterStore.refresh();
        });
        this.hide();
    }

    protected onSplit(): void {
        const issue = this.issue();
        if (!issue) return;
        this.splitRequested.emit(issue);
        this.hide();
    }
}
