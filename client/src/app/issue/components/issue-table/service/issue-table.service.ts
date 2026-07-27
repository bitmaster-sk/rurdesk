import { DestroyRef, Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Observable, Subject, forkJoin, merge } from 'rxjs';
import { filter as rxFilter, first, map, switchMap, tap } from 'rxjs/operators';

import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { IssueService } from '../../../issue.service';
import { Issue } from '../../../model/issue.model';
import { CreateIssueRelationDto, ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { IssueRelationApi } from '../../../api/issue-relation.api.service';
import { IssueRelationRow, IssueTableRow } from '../entity/issue-table-row.entity';
import { CursorPager } from '../../../util/cursor-pager';

const RELATION_LABEL_KEYS: Record<string, string> = {
    child: 'RELATION.CHILD',
    parent: 'RELATION.PARENT',
    duplicates: 'RELATION.DUPLICATES',
    relates_to: 'RELATION.RELATES_TO',
    schedules: 'RELATION.SCHEDULES',
    scheduled_by: 'RELATION.SCHEDULED_BY',
    starts_with: 'RELATION.STARTS_WITH',
    finishes_with: 'RELATION.FINISHES_WITH',
    triggers_end_of: 'RELATION.TRIGGERS_END_OF',
    end_triggered_by: 'RELATION.END_TRIGGERED_BY'
};

@Injectable()
export class IssueTableService {
    private readonly issueService = inject(IssueService);
    private readonly stateStore = inject(StateStore);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly relationApi = inject(IssueRelationApi);
    private readonly settings = inject(SettingsStore);
    private readonly destroyRef = inject(DestroyRef);

    private readonly refresh$ = new Subject<void>();

    private currentFilter: IssuesFilter | null = null;

    // Store snapshots captured per reset (project-scoped maps).
    private readonly severities = signal<Map<number, IssueSeverity>>(new Map());
    private readonly states = signal<Map<number, IssueState>>(new Map());
    private readonly users = signal<Map<number, User>>(new Map());

    // Per-row relations, loaded lazily on expand (keyed by idIssuePublic).
    private readonly relationsMap = signal<Map<number, IssueRelationRow[]>>(new Map());

    private readonly pager = new CursorPager((cursor, limit) =>
        this.issueService.loadIssuesPage$(
            this.currentFilter!,
            limit ?? this.settings.tablePageSize(),
            cursor
        )
    );

    public readonly total = this.pager.total;
    public readonly isLoading = this.pager.isLoading;

    public readonly rows = computed<IssueTableRow[]>(() => {
        const severities = this.severities();
        const states = this.states();
        const users = this.users();
        const relations = this.relationsMap();
        return this.pager.items().map(issue => ({
            issue,
            severity: issue.idSeverity != null ? severities.get(issue.idSeverity) : undefined,
            state: issue.idState != null ? states.get(issue.idState) : undefined,
            assigned: issue.assignedTo != null ? users.get(issue.assignedTo) : undefined,
            relations: relations.get(issue.idIssuePublic!) ?? []
        }));
    });

    constructor() {
        // Drop any leftover filter from a previously-mounted view so we don't fire a
        // stale load before this view's setInitialFilter runs.
        this.issueFilterStore.clear();

        // A filter/order change resets to page 1; a data refresh (store refresh() from a
        // quick-action edit, or an internal relation change) keeps the loaded pages so the
        // user doesn't lose their "Load more" progress.
        const filterChange$ = this.issueFilterStore.actualFilterChange$.pipe(
            map(({ filter, refresh }) => ({ filter, preserve: refresh }))
        );
        const internalRefresh$ = this.refresh$.pipe(
            map(() => ({ filter: this.currentFilter, preserve: true }))
        );

        merge(filterChange$, internalRefresh$)
            .pipe(
                rxFilter((e): e is { filter: IssuesFilter; preserve: boolean } => e.filter != null),
                switchMap(({ filter, preserve }) =>
                    forkJoin([
                        this.severityStore.severitiesMapByProject$(filter.idProject).pipe(first()),
                        this.stateStore.statesMapByProject$(filter.idProject).pipe(first()),
                        this.projectMemberStore.usersMap$.pipe(first())
                    ]).pipe(map(([sev, st, us]) => ({ filter, preserve, sev, st, us })))
                ),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(({ filter, preserve, sev, st, us }) => {
                this.currentFilter = filter;
                this.severities.set(sev);
                this.states.set(st);
                this.users.set(us);
                this.relationsMap.set(new Map());
                if (preserve) {
                    this.pager.refetchExtent();
                } else {
                    this.pager.reset();
                }
            });
    }

    public canLoadMore(): boolean {
        return this.pager.canLoadMore();
    }

    public loadMore(): void {
        this.pager.loadMore();
    }

    // Lazily fetch a single issue's relations (called when a row is expanded).
    public loadRelationsFor(idProject: number, idIssuePublic: number): void {
        this.relationApi.loadForIssue$(idProject, idIssuePublic).subscribe(dtos => {
            const rows = this.toIssueRelationRows(dtos, idIssuePublic);
            this.relationsMap.update(m => new Map(m).set(idIssuePublic, rows));
        });
    }

    public insertRelation$(
        idProject: number,
        from: Issue,
        to: Issue,
        relationType: string,
        subType: string | null,
        lagMinutes: number | null
    ): Observable<void> {
        const isHierarchyChild = relationType === 'hierarchy' && subType === 'child';
        const apiFrom = isHierarchyChild ? to : from;
        const apiTo = isHierarchyChild ? from : to;
        const dto: CreateIssueRelationDto = {
            idIssuePublicTo: apiTo.idIssuePublic!,
            relationType,
            ...(subType && relationType !== 'hierarchy' ? { relationSubType: subType } : {}),
            ...(lagMinutes != null ? { lagMinutes } : {})
        };
        return this.relationApi.insert$(idProject, apiFrom.idIssuePublic!, dto).pipe(
            tap(() => {
                this.loadRelationsFor(idProject, from.idIssuePublic!);
                this.refresh$.next();
            }),
            map(() => undefined)
        );
    }

    public deleteRelation$(
        idProject: number,
        idIssuePublic: number,
        idIssueRelation: number
    ): Observable<void> {
        return this.relationApi
            .delete$(idProject, idIssuePublic, idIssueRelation)
            .pipe(tap(() => this.loadRelationsFor(idProject, idIssuePublic)));
    }

    private toIssueRelationRows(
        relations: ReadIssueRelationDto[],
        idIssuePublic: number
    ): IssueRelationRow[] {
        const severities = this.severities();
        const states = this.states();
        const users = this.users();
        const out: IssueRelationRow[] = [];
        for (const rel of relations) {
            const ref = rel.direction === 'outbound' ? rel.to : rel.from;
            const owner =
                rel.direction === 'outbound' ? rel.from.idIssuePublic : rel.to.idIssuePublic;
            if (owner !== idIssuePublic) continue;
            out.push({
                idIssueRelation: rel.idIssueRelation,
                labelKey: RELATION_LABEL_KEYS[rel.label] ?? rel.label,
                ref,
                severity: ref.idSeverity != null ? severities.get(ref.idSeverity) : undefined,
                state: ref.idState != null ? states.get(ref.idState) : undefined,
                assigned: ref.assignedTo != null ? users.get(ref.assignedTo) : undefined
            });
        }
        return out;
    }
}
