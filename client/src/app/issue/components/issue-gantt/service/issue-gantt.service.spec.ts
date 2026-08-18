import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { IssueGanttService } from './issue-gantt.service';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { IssuesPage } from '../../../model/issues-page.model';
import { IssueService } from '../../../issue.service';
import { IssueRelationApi } from '../../../api/issue-relation.api.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { StateStore } from 'src/app/state/store/state.store';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { Issue } from '../../../model/issue.model';
import { ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { IssueRelationType } from 'src/app/issue/constants/issue-relation-type.enum';
import { IssueRelationDirection } from 'src/app/issue/constants/issue-relation-direction.enum';
import { topologicalSort } from './gantt-order.util';

function initialFilter(): IssuesFilter {
    return { idProject: 1, orderColumn: 'idIssuePublic', orderDirection: 'desc' };
}

// IssueGanttService uses inject() for 6 deps; only actualFilter$ is touched at
// construction (the rest stay lazy inside switchMap). topologicalSort is pure, so we
// build the service with stub deps and call the private method directly.
function buildService(): IssueGanttService {
    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => {} } },
            { provide: SettingsStore, useValue: { ganttBacklogPageSize: () => 30 } },
            {
                provide: IssueFilterStore,
                useValue: {
                    clear: () => {},
                    actualFilter$: of({ idProject: 1 }),
                    actualFilterChange$: of({ filter: { idProject: 1 }, refresh: false })
                }
            },
            {
                provide: IssueService,
                useValue: {
                    loadIssues: () => of([]),
                    loadIssuesPage$: () => of({ items: [], nextCursor: null, total: 0 })
                }
            },
            { provide: IssueRelationApi, useValue: { load$: () => of([]) } },
            { provide: SeverityStore, useValue: { severitiesMapByProject$: () => of(new Map()) } },
            { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
            { provide: StateStore, useValue: { statesMapByProject$: () => of(new Map()) } }
        ]
    });
    return runInInjectionContext(injector, () => new IssueGanttService());
}

function makeIssue(id: number, scheduledAt: string): Issue {
    return {
        idIssue: id,
        idIssuePublic: id,
        idProject: 1,
        idState: null,
        idSeverity: null,
        title: `Issue ${id}`,
        description: '',
        tracked: 0,
        scheduledAt: new Date(scheduledAt)
    };
}

function makeRelation(fromId: number, toId: number): ReadIssueRelationDto {
    return {
        idIssueRelation: fromId * 100 + toId,
        relationType: IssueRelationType.Schedule,
        relationSubType: null,
        lagMinutes: null,
        direction: IssueRelationDirection.Outbound,
        label: '',
        inverseLabel: '',
        from: {
            idIssuePublic: fromId,
            title: '',
            idSeverity: null,
            idState: null,
            assignedTo: null,
            updateAt: '',
            qualityScore: null
        },
        to: {
            idIssuePublic: toId,
            title: '',
            idSeverity: null,
            idState: null,
            assignedTo: null,
            updateAt: '',
            qualityScore: null
        },
        createdAt: '',
        createdBy: 1
    };
}

// topologicalSort now lives in the pure util; test it directly.
function topoSort(
    _svc: IssueGanttService,
    issues: Issue[],
    relations: ReadIssueRelationDto[]
): Issue[] {
    return topologicalSort(issues, relations);
}

describe('IssueGanttService — topologicalSort', () => {
    it('orders A → B → C in dependency order (not date order)', () => {
        const svc = buildService();
        // Dates intentionally out of dependency order.
        const a = makeIssue(1, '2026-04-03T00:00:00Z');
        const b = makeIssue(2, '2026-04-01T00:00:00Z');
        const c = makeIssue(3, '2026-04-02T00:00:00Z');
        const sorted = topoSort(svc, [a, b, c], [makeRelation(1, 2), makeRelation(2, 3)]);

        expect(sorted.map(i => i.idIssuePublic)).toEqual([1, 2, 3]);
    });

    it('falls back to scheduledAt order for unconnected issues', () => {
        const svc = buildService();
        const x = makeIssue(1, '2026-04-05T00:00:00Z');
        const y = makeIssue(2, '2026-04-01T00:00:00Z');
        const sorted = topoSort(svc, [x, y], []);

        expect(sorted.map(i => i.idIssuePublic)).toEqual([2, 1]); // Y (earlier) first
    });

    it('handles a DAG with multiple roots', () => {
        const svc = buildService();
        const a = makeIssue(1, '2026-04-01T00:00:00Z');
        const b = makeIssue(2, '2026-04-02T00:00:00Z');
        const c = makeIssue(3, '2026-04-05T00:00:00Z');
        const sorted = topoSort(svc, [a, b, c], [makeRelation(1, 3), makeRelation(2, 3)]);

        expect(sorted.map(i => i.idIssuePublic)).toEqual([1, 2, 3]);
    });
});

// Backlog pagination survives a refresh() (e.g. after a backlog task is scheduled).
// Regression: refresh() used to re-emit the filter and snap the backlog back to page 1,
// unloading every "Load more" page the user had fetched.
describe('IssueGanttService — backlog refresh keeps loaded pages', () => {
    function backlogItems(n: number): Issue[] {
        return Array.from({ length: n }, (_, i): Issue => ({
            idIssue: i + 1,
            idIssuePublic: i + 1,
            idProject: 1,
            idState: null,
            idSeverity: null,
            title: `Backlog ${i + 1}`,
            description: '',
            tracked: 0,
            scheduledAt: null
        }));
    }

    function buildWithStore(
        store: IssueFilterStore,
        pageResponder: (limit: number, cursor: string | null) => IssuesPage,
        calls: { limit: number; cursor: string | null }[]
    ): IssueGanttService {
        const injector = Injector.create({
            providers: [
                { provide: DestroyRef, useValue: { onDestroy: () => () => {} } },
                { provide: SettingsStore, useValue: { ganttBacklogPageSize: () => 30 } },
                { provide: IssueFilterStore, useValue: store },
                {
                    provide: IssueService,
                    useValue: {
                        loadIssues: () => of([]),
                        loadIssuesPage$: (_f: unknown, limit: number, cursor: string | null) => {
                            calls.push({ limit, cursor });
                            return of(pageResponder(limit, cursor));
                        }
                    }
                },
                { provide: IssueRelationApi, useValue: { load$: () => of([]) } },
                {
                    provide: SeverityStore,
                    useValue: { severitiesMapByProject$: () => of(new Map()) }
                },
                { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
                { provide: StateStore, useValue: { statesMapByProject$: () => of(new Map()) } }
            ]
        });
        return runInInjectionContext(injector, () => new IssueGanttService());
    }

    function backlogLength(svc: IssueGanttService): number {
        return (
            svc as unknown as { backlogIssues$: { getValue(): Issue[] } }
        ).backlogIssues$.getValue().length;
    }

    it('refetches the whole loaded window on refresh instead of resetting to page 1', () => {
        const store = new IssueFilterStore();
        const calls: { limit: number; cursor: string | null }[] = [];
        const pageResponder = (limit: number, cursor: string | null): IssuesPage => {
            if (cursor === null && limit === 30)
                return { items: backlogItems(30), nextCursor: 'c1', total: 100 };
            if (cursor === 'c1') return { items: backlogItems(30), nextCursor: 'c2', total: 100 };
            // refresh: one task was scheduled away, window shrinks by one but stays large.
            if (cursor === null && limit === 60)
                return { items: backlogItems(59), nextCursor: 'c2b', total: 99 };
            return { items: [], nextCursor: null, total: 0 };
        };
        const svc = buildWithStore(store, pageResponder, calls);

        store.setInitialFilter(initialFilter()); // page 1
        svc.loadMoreBacklog(); // page 2 → 60 loaded
        expect(backlogLength(svc)).toBe(60);

        store.refresh();

        expect(backlogLength(svc)).toBe(59); // preserved extent, NOT 30
        expect(calls).toContainEqual({ limit: 60, cursor: null });
    });

    it('still resets to page 1 on a real filter change', () => {
        const store = new IssueFilterStore();
        const calls: { limit: number; cursor: string | null }[] = [];
        const pageResponder = (): IssuesPage => ({
            items: backlogItems(30),
            nextCursor: 'c1',
            total: 100
        });
        const svc = buildWithStore(store, pageResponder, calls);

        store.setInitialFilter(initialFilter());
        svc.loadMoreBacklog();
        calls.length = 0;

        store.setFilter({ title: 'abc' });

        // A filter change fetches page 1 with cursor null and the default page size.
        expect(calls).toEqual([{ limit: 30, cursor: null }]);
        expect(backlogLength(svc)).toBe(30);
    });
});
