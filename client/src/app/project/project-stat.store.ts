import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, EMPTY, combineLatest } from 'rxjs';
import { catchError, filter, map, switchMap } from 'rxjs/operators';
import { IssueService } from '../issue/issue.service';
import { Issue } from '../issue/model/issue.model';
import { IssueSeverity } from '../severity/model/issue-severity.model';
import { SeverityStore } from '../severity/store/severity.store';
import { IssueState } from '../state/model/issue-state.model';
import { StateStore } from '../state/store/state.store';
import { Project } from './model/project.model';
import { ProjectStore } from './project.store';

interface ProjectStats {
    totalIssues: number;
    totalEstimatedSeconds: number;
    totalTrackedSeconds: number;
    issuesByState: Map<number, [IssueState, number]>;
    issuesBySeverity: Map<number, [IssueSeverity, number]>;
    openIssuesByAssignee: Map<number | null, number>;
}

@Injectable({
    providedIn: 'root'
})
export class ProjectStatStore {
    private readonly issueService = inject(IssueService);
    private readonly projectStore = inject(ProjectStore);
    private readonly stateStore = inject(StateStore);
    private readonly severityStore = inject(SeverityStore);

    private totalEstimatedSeconds = new BehaviorSubject<number>(0);

    public totalEstimatedSeconds$ = this.totalEstimatedSeconds.asObservable();

    private totalTrackedSeconds = new BehaviorSubject<number>(0);

    public totalTrackedSeconds$ = this.totalTrackedSeconds.asObservable();

    private issuesByState = new BehaviorSubject<[IssueState, number][] | null>(null);

    public issuesByState$ = this.issuesByState
        .asObservable()
        .pipe(filter(issuesByState => !!issuesByState));

    private issuesBySeverity = new BehaviorSubject<[IssueSeverity, number][] | null>(null);

    public issuesBySeverity$ = this.issuesBySeverity
        .asObservable()
        .pipe(filter(issuesBySeverity => !!issuesBySeverity));

    private openIssuesByAssignee = new BehaviorSubject<
        { idAssignedTo: number | null; count: number }[] | null
    >(null);

    public openIssuesByAssignee$ = this.openIssuesByAssignee.asObservable().pipe(filter(v => !!v));

    public constructor() {
        this.initialize();
    }

    private initialize(): void {
        combineLatest([
            this.projectStore.project$,
            this.stateStore.states$,
            this.severityStore.severities$
        ])
            .pipe(
                map(
                    ([project, states, severities]) =>
                        [
                            project,
                            states.filter(s => s.idProject === project.idProject),
                            severities.filter(s => s.idProject === project.idProject)
                        ] as [Project, IssueState[], IssueSeverity[]]
                ),
                switchMap(
                    ([project, states, severities]: [Project, IssueState[], IssueSeverity[]]) =>
                        this.issueService
                            .loadIssues({
                                idProject: project.idProject,
                                idsSeverity: [],
                                severityUnset: true,
                                idsState: [],
                                stateUnset: true,
                                idsAssignedTo: [],
                                assignedToUnset: true,
                                createAtFrom: null,
                                createAtTo: null,
                                updateAtFrom: null,
                                updateAtTo: null,
                                orderColumn: 'updateAt',
                                orderDirection: 'desc'
                            })
                            // Contain the failure to this one fetch. The error must not reach
                            // the outer subscription: this store is a root singleton wired up
                            // once in the constructor, so an errored stream would leave every
                            // project statistic frozen for the rest of the session with
                            // nothing to resubscribe it. Skipping the emission keeps the last
                            // good numbers on screen until the next project/state/severity
                            // change retriggers the fetch.
                            .pipe(
                                map(
                                    issues =>
                                        [issues, states, severities] as [
                                            Issue[],
                                            IssueState[],
                                            IssueSeverity[]
                                        ]
                                ),
                                catchError(err => {
                                    console.error('loading issues for project stats failed', err);
                                    return EMPTY;
                                })
                            )
                )
            )
            .subscribe(([issues, states, severities]: [Issue[], IssueState[], IssueSeverity[]]) =>
                this.createStats(issues, states, severities)
            );
    }

    private createStats(issues: Issue[], states: IssueState[], severities: IssueSeverity[]): void {
        let stats = this.createEmptyStats(states, severities);
        issues.forEach(issue => {
            stats = this.issueToStats(stats, issue);
        });

        this.pushStats(stats);
    }

    private issueToStats(stats: ProjectStats, issue: Issue): ProjectStats {
        stats.totalIssues++;

        if ((issue.estimated ?? 0) > 0) {
            stats.totalEstimatedSeconds += issue.estimated ?? 0;
        }

        if (issue.tracked > 0) {
            stats.totalTrackedSeconds += issue.tracked;
        }

        if (issue.idState) {
            const byState = stats.issuesByState.get(issue.idState);
            if (byState) {
                const [state, count] = byState;
                stats.issuesByState.set(issue.idState, [state, count + 1]);
            }
        }

        if (issue.idSeverity) {
            const bySeverity = stats.issuesBySeverity.get(issue.idSeverity);
            if (bySeverity) {
                const [severity, count] = bySeverity;
                stats.issuesBySeverity.set(issue.idSeverity, [severity, count + 1]);
            }
        }

        // Count open issues (non-final state) per assignee
        const stateEntry = issue.idState ? stats.issuesByState.get(issue.idState) : undefined;
        const isOpen = !stateEntry?.[0].final;

        if (isOpen) {
            const key = issue.assignedTo ?? null;
            stats.openIssuesByAssignee.set(key, (stats.openIssuesByAssignee.get(key) ?? 0) + 1);
        }

        return stats;
    }

    private pushStats(stats: ProjectStats): void {
        this.totalEstimatedSeconds.next(stats.totalEstimatedSeconds);
        this.totalTrackedSeconds.next(stats.totalTrackedSeconds);

        const ibs: [IssueState, number][] = [];
        stats.issuesByState.forEach(issueByState => ibs.push(issueByState));
        this.issuesByState.next(ibs);

        const ibse: [IssueSeverity, number][] = [];
        stats.issuesBySeverity.forEach(issueBySeverity => ibse.push(issueBySeverity));
        this.issuesBySeverity.next(ibse);

        // Emit workload sorted by count desc, unassigned last. Presentation
        // (name/avatar) is resolved in the page from the project-member store.
        const workload = Array.from(stats.openIssuesByAssignee.entries())
            .map(([idAssignedTo, count]) => ({ idAssignedTo, count }))
            .sort((a, b) => {
                if (a.idAssignedTo === null) return 1;
                if (b.idAssignedTo === null) return -1;
                return b.count - a.count;
            });
        this.openIssuesByAssignee.next(workload);
    }

    private createEmptyStats(states: IssueState[], severities: IssueSeverity[]): ProjectStats {
        const issuesByState = new Map<number, [IssueState, number]>();
        states.forEach(state => issuesByState.set(state.idState, [state, 0]));

        const issuesBySeverity = new Map<number, [IssueSeverity, number]>();
        severities.forEach(severity => issuesBySeverity.set(severity.idSeverity, [severity, 0]));

        return {
            totalIssues: 0,
            totalEstimatedSeconds: 0,
            totalTrackedSeconds: 0,
            issuesByState,
            issuesBySeverity,
            openIssuesByAssignee: new Map()
        };
    }
}
