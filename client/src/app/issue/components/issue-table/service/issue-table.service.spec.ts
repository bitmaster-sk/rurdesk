// @vitest-environment jsdom
import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { IssueTableService } from './issue-table.service';
import { IssueService } from '../../../issue.service';
import { StateStore } from 'src/app/state/store/state.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { IssueRelationApi } from '../../../api/issue-relation.api.service';
import { Issue } from '../../../model/issue.model';
import { ReadIssueRelationDto } from '../../../model/issue-relation.model';
import { IssueRelationType } from '../../../constants/issue-relation-type.enum';
import { IssueRelationDirection } from '../../../constants/issue-relation-direction.enum';

function initialFilter(): IssuesFilter {
    return { idProject: 1, orderColumn: 'idIssuePublic', orderDirection: 'desc' };
}

function makeIssue(over: Partial<Issue>): Issue {
    return {
        idIssue: 1,
        idIssuePublic: 1,
        idProject: 1,
        idState: 1,
        idSeverity: 1,
        title: 'T',
        description: '',
        tracked: 0,
        assignedTo: 10,
        relationCount: 0,
        ...over
    };
}

function buildService(issues: Issue[], relations: ReadIssueRelationDto[] = []): IssueTableService {
    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => {} } },
            { provide: SettingsStore, useValue: { tablePageSize: () => 50 } },
            {
                provide: IssueService,
                useValue: {
                    loadIssuesPage$: () =>
                        of({ items: issues, nextCursor: null, total: issues.length })
                }
            },
            {
                provide: StateStore,
                useValue: {
                    statesMapByProject$: () => of(new Map([[1, { idState: 1, name: 'Todo' }]]))
                }
            },
            {
                provide: ProjectMemberStore,
                useValue: { usersMap$: of(new Map([[10, { idUser: 10, name: 'Alice' }]])) }
            },
            {
                provide: SeverityStore,
                useValue: {
                    severitiesMapByProject$: () =>
                        of(new Map([[1, { idSeverity: 1, title: 'Low' }]]))
                }
            },
            {
                provide: IssueFilterStore,
                useValue: {
                    clear: () => {},
                    actualFilter$: of({ idProject: 1 }),
                    actualFilterChange$: of({ filter: { idProject: 1 }, refresh: false })
                }
            },
            { provide: IssueRelationApi, useValue: { loadForIssue$: () => of(relations) } }
        ]
    });
    return runInInjectionContext(injector, () => new IssueTableService());
}

function relation(fromId: number, toId: number, label: string): ReadIssueRelationDto {
    return {
        idIssueRelation: 99,
        relationType: IssueRelationType.Schedule,
        relationSubType: null,
        lagMinutes: null,
        direction: IssueRelationDirection.Outbound,
        label,
        inverseLabel: '',
        from: {
            idIssuePublic: fromId,
            title: '',
            idSeverity: 1,
            idState: 1,
            assignedTo: 10,
            updateAt: '',
            qualityScore: null
        },
        to: {
            idIssuePublic: toId,
            title: '',
            idSeverity: 1,
            idState: 1,
            assignedTo: 10,
            updateAt: '',
            qualityScore: null
        },
        createdAt: '',
        createdBy: 1
    };
}

describe('IssueTableService — rows', () => {
    beforeEach(() => localStorage.clear());

    it('maps each issue with its severity, state and assignee (no relations until expanded)', () => {
        const svc = buildService([makeIssue({ idIssuePublic: 1 })]);
        const rows = svc.rows();
        expect(rows).toHaveLength(1);
        expect(rows[0].severity?.title).toBe('Low');
        expect(rows[0].state?.name).toBe('Todo');
        expect(rows[0].assigned?.name).toBe('Alice');
        expect(rows[0].relations).toEqual([]);
    });

    it('lazily attaches a row’s relations, mapping the label key', () => {
        const svc = buildService([makeIssue({ idIssuePublic: 1 })], [relation(1, 2, 'child')]);
        svc.loadRelationsFor(1, 1);
        const rows = svc.rows();
        expect(rows[0].relations).toHaveLength(1);
        expect(rows[0].relations[0].labelKey).toBe('RELATION.CHILD');
        expect(rows[0].relations[0].ref.idIssuePublic).toBe(2);
    });
});

// A store refresh() (e.g. a quick-action edit) must not throw away the "Load more" pages
// the user has fetched. Regression: the table reset to page 1 on every actualFilter$ emit.
describe('IssueTableService — refresh keeps loaded pages', () => {
    function issues(n: number): Issue[] {
        return Array.from({ length: n }, (_, i) => makeIssue({ idIssuePublic: i + 1 }));
    }

    function buildWithStore(
        store: IssueFilterStore,
        pageResponder: (
            limit: number,
            cursor: string | null
        ) => { items: Issue[]; nextCursor: string | null; total: number },
        calls: { limit: number; cursor: string | null }[]
    ): IssueTableService {
        const injector = Injector.create({
            providers: [
                { provide: DestroyRef, useValue: { onDestroy: () => () => {} } },
                { provide: SettingsStore, useValue: { tablePageSize: () => 50 } },
                {
                    provide: IssueService,
                    useValue: {
                        loadIssuesPage$: (_f: unknown, limit: number, cursor: string | null) => {
                            calls.push({ limit, cursor });
                            return of(pageResponder(limit, cursor));
                        }
                    }
                },
                {
                    provide: StateStore,
                    useValue: {
                        statesMapByProject$: () => of(new Map([[1, { idState: 1, name: 'Todo' }]]))
                    }
                },
                {
                    provide: ProjectMemberStore,
                    useValue: { usersMap$: of(new Map([[10, { idUser: 10, name: 'Alice' }]])) }
                },
                {
                    provide: SeverityStore,
                    useValue: {
                        severitiesMapByProject$: () =>
                            of(new Map([[1, { idSeverity: 1, title: 'Low' }]]))
                    }
                },
                { provide: IssueFilterStore, useValue: store },
                { provide: IssueRelationApi, useValue: { loadForIssue$: () => of([]) } }
            ]
        });
        return runInInjectionContext(injector, () => new IssueTableService());
    }

    it('refetches the loaded window on refresh instead of resetting to page 1', () => {
        const store = new IssueFilterStore();
        const calls: { limit: number; cursor: string | null }[] = [];
        const responder = (limit: number, cursor: string | null) => {
            if (cursor === null && limit === 50)
                return { items: issues(50), nextCursor: 'c1', total: 100 };
            if (cursor === 'c1') return { items: issues(50), nextCursor: 'c2', total: 100 };
            if (cursor === null && limit === 100)
                return { items: issues(99), nextCursor: 'c2b', total: 99 };
            return { items: [], nextCursor: null, total: 0 };
        };
        const svc = buildWithStore(store, responder, calls);

        store.setInitialFilter({ idProject: 1 } as never); // page 1 → 50
        svc.loadMore(); // page 2 → 100
        expect(svc.rows()).toHaveLength(100);

        store.refresh();

        expect(svc.rows()).toHaveLength(99); // preserved extent, NOT 50
        expect(calls).toContainEqual({ limit: 100, cursor: null });
    });
});
