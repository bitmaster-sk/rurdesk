import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { BehaviorSubject, combineLatest, switchMap, map, shareReplay } from 'rxjs';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { IssueService } from '../../../issue.service';
import { IssueRelationApi } from '../../../api/issue-relation.api.service';
import { Issue } from '../../../model/issue.model';
import { ExtendedIssue } from '../../../model/extended-issue.model';
import { ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { StateStore } from 'src/app/state/store/state.store';
import { User } from 'src/app/auth/model/user.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueRelationType } from 'src/app/issue/constants/issue-relation-type.enum';
import { orderScheduled } from './gantt-order.util';

@Injectable()
export class IssueGanttService {
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly issueService = inject(IssueService);
    private readonly issueRelationApi = inject(IssueRelationApi);
    private readonly severityStore = inject(SeverityStore);
    private readonly memberStore = inject(ProjectMemberStore);
    private readonly stateStore = inject(StateStore);
    private readonly settings = inject(SettingsStore);
    private readonly destroyRef = inject(DestroyRef);

    // Scheduled issues
    private readonly scheduledIssues$ = this.issueFilterStore.actualFilter$.pipe(
        switchMap(filter => this.issueService.loadIssues(filter)),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    // Backlog issues (scheduledAt IS NULL) — paginated via "Load more".
    private readonly backlogIssues$ = new BehaviorSubject<Issue[]>([]);
    private backlogFilter: IssuesFilter | null = null;
    private backlogCursor: string | null = null;
    public readonly backlogHasMore = signal(false);
    public readonly backlogLoading = signal(false);

    constructor() {
        // Drop any leftover filter from a previously-mounted view so we don't fire a
        // stale load before this view's setInitialFilter runs.
        this.issueFilterStore.clear();

        this.issueFilterStore.actualFilterChange$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(({ filter, refresh }) => {
                this.backlogFilter = {
                    ...filter,
                    scheduledAtFrom: undefined,
                    scheduledAtTo: undefined,
                    scheduledAtUnset: true,
                    orderColumn: 'createAt',
                    orderDirection: 'desc'
                };
                if (refresh) {
                    // Data refresh (e.g. a backlog task was scheduled): keep the pages the
                    // user already loaded instead of resetting to page 1.
                    this.refetchBacklogExtent();
                } else {
                    // Real filter/order change: start over from page 1.
                    this.backlogCursor = null;
                    this.fetchBacklogPage(true);
                }
            });
    }

    // Re-fetch the currently loaded backlog window as a single request so a refresh keeps
    // the user's scroll extent. A task that was scheduled drops out of the backlog, so the
    // refreshed window naturally back-fills with the next item.
    private refetchBacklogExtent(): void {
        if (!this.backlogFilter) return;
        const loaded = this.backlogIssues$.getValue().length;
        const limit = Math.max(loaded, this.settings.ganttBacklogPageSize());
        this.backlogLoading.set(true);
        this.issueService.loadIssuesPage$(this.backlogFilter, limit, null).subscribe({
            next: page => {
                this.backlogIssues$.next(page.items);
                this.backlogCursor = page.nextCursor;
                this.backlogHasMore.set(page.nextCursor !== null);
                this.backlogLoading.set(false);
            },
            // loadMoreBacklog() is gated on backlogLoading — leaving it set kills
            // backlog paging until the view remounts.
            error: () => this.backlogLoading.set(false)
        });
    }

    private fetchBacklogPage(reset: boolean): void {
        if (!this.backlogFilter) return;
        this.backlogLoading.set(true);
        this.issueService
            .loadIssuesPage$(
                this.backlogFilter,
                this.settings.ganttBacklogPageSize(),
                reset ? null : this.backlogCursor
            )
            .subscribe({
                next: page => {
                    this.backlogIssues$.next(
                        reset ? page.items : [...this.backlogIssues$.getValue(), ...page.items]
                    );
                    this.backlogCursor = page.nextCursor;
                    this.backlogHasMore.set(page.nextCursor !== null);
                    this.backlogLoading.set(false);
                },
                error: () => this.backlogLoading.set(false)
            });
    }

    public loadMoreBacklog(): void {
        if (this.backlogCursor === null || this.backlogLoading()) return;
        this.fetchBacklogPage(false);
    }

    // Relations for the project
    private readonly serverRelations$ = this.issueFilterStore.actualFilter$.pipe(
        switchMap(filter => this.issueRelationApi.load$(filter.idProject!)),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    // Locally added or removed relations (incremental updates without full reload)
    private readonly _additionalRelations = new BehaviorSubject<ReadIssueRelationDto[]>([]);
    private readonly _deletedRelationIds = new BehaviorSubject<Set<number>>(new Set());

    private readonly relations$ = combineLatest([
        this.serverRelations$,
        this._additionalRelations,
        this._deletedRelationIds
    ]).pipe(
        map(([serverRelations, additionalRelations, deletedIds]) => {
            const serverIds = new Set(serverRelations.map(r => r.idIssueRelation));
            return [
                ...serverRelations.filter(r => !deletedIds.has(r.idIssueRelation)),
                ...additionalRelations.filter(
                    r => !serverIds.has(r.idIssueRelation) && !deletedIds.has(r.idIssueRelation)
                )
            ];
        }),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    public addRelations(relations: ReadIssueRelationDto[]): void {
        const current = this._additionalRelations.getValue();
        const existingIds = new Set(current.map(r => r.idIssueRelation));
        const toAdd = relations.filter(r => !existingIds.has(r.idIssueRelation));
        if (toAdd.length > 0) {
            this._additionalRelations.next([...current, ...toAdd]);
        }
    }

    public removeRelation(idIssueRelation: number): void {
        const deletedIds = new Set(this._deletedRelationIds.getValue());
        deletedIds.add(idIssueRelation);
        this._deletedRelationIds.next(deletedIds);
        this._additionalRelations.next(
            this._additionalRelations.getValue().filter(r => r.idIssueRelation !== idIssueRelation)
        );
    }

    // Metadata maps
    private readonly metadata$ = this.issueFilterStore.actualFilter$.pipe(
        switchMap(filter =>
            combineLatest([
                this.severityStore.severitiesMapByProject$(filter.idProject!),
                this.stateStore.statesMapByProject$(filter.idProject!),
                this.memberStore.usersMap$
            ])
        ),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    // Combined data as signals
    public readonly data$ = combineLatest([
        this.scheduledIssues$,
        this.backlogIssues$,
        this.relations$,
        this.metadata$
    ]).pipe(
        map(([scheduled, backlog, relations, [severities, states, users]]) => {
            const scheduleRelations = relations.filter(
                r => r.relationType === IssueRelationType.Schedule
            );
            const scheduledOnly = scheduled.filter(i => i.scheduledAt != null);
            const sortedScheduled = orderScheduled(scheduledOnly, scheduleRelations);
            return {
                scheduledTasks: sortedScheduled.map(issue =>
                    this.buildExtendedIssue(issue, severities, states, users)
                ),
                backlogTasks: backlog.map(issue =>
                    this.buildExtendedIssue(issue, severities, states, users)
                ),
                relations: scheduleRelations
            };
        }),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    private buildExtendedIssue(
        issue: Issue,
        severities: Map<number, IssueSeverity>,
        states: Map<number, IssueState>,
        users: Map<number, User>
    ): ExtendedIssue {
        return {
            ...issue,
            state: issue.idState != null ? states.get(issue.idState) : undefined,
            severity: issue.idSeverity != null ? severities.get(issue.idSeverity) : undefined,
            assignedToUser: issue.assignedTo != null ? users.get(issue.assignedTo) : undefined
        };
    }
}
