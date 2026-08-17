import { Injector, runInInjectionContext } from '@angular/core';
import { BehaviorSubject, of, throwError } from 'rxjs';
import { ProjectStatStore } from './project-stat.store';
import { IssueService } from '../issue/issue.service';
import { ProjectStore } from './project.store';
import { StateStore } from '../state/store/state.store';
import { SeverityStore } from '../severity/store/severity.store';
import { Issue } from '../issue/model/issue.model';

const states = [
    { idState: 1, idProject: 1, name: 'Todo', final: false },
    { idState: 2, idProject: 1, name: 'Done', final: true }
];
const severities = [{ idSeverity: 1, idProject: 1, title: 'Low' }];

function issue(over: Partial<Issue>): Issue {
    return {
        idState: 1,
        idSeverity: 1,
        estimated: 0,
        tracked: 0,
        assignedTo: null,
        ...over
    };
}

function buildStore(
    issueService: IssueService,
    projectStore: ProjectStore,
    stateStore: StateStore,
    severityStore: SeverityStore
): ProjectStatStore {
    const injector = Injector.create({
        providers: [
            { provide: IssueService, useValue: issueService },
            { provide: ProjectStore, useValue: projectStore },
            { provide: StateStore, useValue: stateStore },
            { provide: SeverityStore, useValue: severityStore }
        ]
    });
    return runInInjectionContext(injector, () => new ProjectStatStore());
}

function build(issues: Issue[]): ProjectStatStore {
    return buildStore(
        { loadIssues: () => of(issues) } as unknown as IssueService,
        { project$: of({ idProject: 1 }) } as unknown as ProjectStore,
        { states$: of(states) } as unknown as StateStore,
        { severities$: of(severities) } as unknown as SeverityStore
    );
}

function latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
    let v!: T;
    obs.subscribe(x => (v = x));
    return v;
}

describe('ProjectStatStore', () => {
    it('sums estimated and tracked seconds (only positive values)', () => {
        const store = build([
            issue({ estimated: 3600, tracked: 1800 }),
            issue({ estimated: 7200, tracked: 0, idState: 2 }),
            issue({ estimated: 0 })
        ]);
        expect(latest(store.totalEstimatedSeconds$)).toBe(10800);
        expect(latest(store.totalTrackedSeconds$)).toBe(1800);
    });

    it('counts issues per state and per severity', () => {
        const store = build([issue({ idState: 1 }), issue({ idState: 1 }), issue({ idState: 2 })]);
        const byState = latest(store.issuesByState$);
        expect(byState.find(([s]) => s.idState === 1)?.[1]).toBe(2);
        expect(byState.find(([s]) => s.idState === 2)?.[1]).toBe(1);
        expect(latest(store.issuesBySeverity$)[0][1]).toBe(3);
    });

    it('computes open-issue workload per assignee, unassigned last', () => {
        const store = build([
            issue({ idState: 1, assignedTo: 10 }), // open, assigned
            issue({ idState: 2, assignedTo: 20 }), // final state → not open
            issue({ idState: 1, assignedTo: null }) // open, unassigned
        ]);
        const workload = latest(store.openIssuesByAssignee$);
        expect(workload.map(w => w.idAssignedTo)).toEqual([10, null]); // unassigned last
        expect(workload.find(w => w.idAssignedTo === 20)).toBeUndefined(); // final not counted
    });

    // The store is a root singleton subscribed once in its constructor: if a
    // failed fetch is allowed to error that subscription, every statistic stays
    // frozen for the rest of the session and nothing resubscribes it.
    it('keeps updating after a failed issue fetch', () => {
        const project$ = new BehaviorSubject({ idProject: 1 });
        let failNext = true;
        const issueService = {
            loadIssues: () => {
                if (failNext) {
                    failNext = false;
                    return throwError(() => new Error('boom'));
                }
                return of([issue({ estimated: 3600, tracked: 1800 })]);
            }
        } as unknown as IssueService;

        const store = new ProjectStatStore(
            issueService,
            { project$ } as unknown as ProjectStore,
            { states$: of(states) } as unknown as StateStore,
            { severities$: of(severities) } as unknown as SeverityStore
        );

        // First fetch failed, so nothing was published yet.
        expect(latest(store.totalEstimatedSeconds$)).toBe(0);

        // A later project emission must still reach the store.
        project$.next({ idProject: 1 });

        expect(latest(store.totalEstimatedSeconds$)).toBe(3600);
        expect(latest(store.totalTrackedSeconds$)).toBe(1800);
    });
});
