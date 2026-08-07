import { Component, inject, OnInit } from '@angular/core';
import { ProjectStore } from '../../project.store';
import { ProjectStatStore } from '../../project-stat.store';
import { combineLatest, merge, Observable, ReplaySubject, Subject } from 'rxjs';
import { distinctUntilChanged, map, switchMap, withLatestFrom } from 'rxjs/operators';
import { PinService } from 'src/app/pin/pin.service';
import { PinDestinationType } from 'src/app/pin/constant/pin-destination-type.enum';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { PinView } from 'src/app/pin/entity/pin-view.entity';
import { StatsChartEntry } from '../../components/project-stats-chart/project-stats-chart.component';
import { WorkloadEntry } from '../../components/workload-bar-list/workload-bar-list.component';
import { ProjectMemberStore } from '../../project-member.store';
import { TranslateService } from '@ngx-translate/core';

@Component({
    selector: 'app-project',
    templateUrl: './project.page.html',
    styleUrls: ['./project.page.scss'],
    standalone: false
})
export class ProjectPage implements OnInit {
    private readonly projectStore = inject(ProjectStore);

    private readonly projectStatStore = inject(ProjectStatStore);

    private readonly projectMemberStore = inject(ProjectMemberStore);

    private readonly sPin = inject(PinService);

    private readonly severityStore = inject(SeverityStore);

    private readonly translate = inject(TranslateService);

    private _loadSignal$ = new ReplaySubject<void>(1);

    public project$ = this.projectStore.project$;

    public totalEstimatedSeconds$ = this.projectStatStore.totalEstimatedSeconds$;

    public totalTrackedSeconds$ = this.projectStatStore.totalTrackedSeconds$;

    public estimatedVsTracked$ = this._loadSignal$.pipe(
        switchMap(() =>
            combineLatest([
                this.projectStatStore.totalEstimatedSeconds$,
                this.projectStatStore.totalTrackedSeconds$,
                this.translate.get(['STATS.ESTIMATED', 'STATS.TRACKED'])
            ]).pipe(
                map(([estimated, tracked, labels]): StatsChartEntry[] => [
                    { name: labels['STATS.ESTIMATED'], value: estimated },
                    { name: labels['STATS.TRACKED'], value: tracked }
                ])
            )
        )
    );

    public workloadByAssignee$ = this._loadSignal$.pipe(
        switchMap(() =>
            combineLatest([
                this.projectStatStore.openIssuesByAssignee$,
                this.projectMemberStore.usersMap$,
                this.translate.get('STATS.UNASSIGNED')
            ]).pipe(
                map(([workload, usersMap, unassignedLabel]): WorkloadEntry[] =>
                    workload.map(w => {
                        if (w.idAssignedTo === null) {
                            return {
                                name: unassignedLabel,
                                count: w.count,
                                isUnassigned: true
                            };
                        }
                        const user = usersMap.get(w.idAssignedTo);
                        return {
                            name: user?.name ?? `#${w.idAssignedTo}`,
                            count: w.count,
                            bgColor: user?.colorAvatarBg,
                            isUnassigned: false
                        };
                    })
                )
            )
        )
    );

    public issuesByState$ = this._loadSignal$.pipe(
        switchMap(() =>
            this.projectStatStore.issuesByState$.pipe(
                map(issuesByState =>
                    issuesByState.map(([state, count]): StatsChartEntry => ({
                        name: state.name,
                        value: count
                    }))
                )
            )
        )
    );

    public issuesBySeverity$ = this._loadSignal$.pipe(
        switchMap(() =>
            this.projectStatStore.issuesBySeverity$.pipe(
                map(issuesBySeverity => ({
                    entries: issuesBySeverity.map(([severity, count]): StatsChartEntry => ({
                        name: severity.title,
                        value: count
                    })),
                    colors: issuesBySeverity.map(([severity]) => severity.color)
                }))
            )
        )
    );

    private _pinRefreshSignal$ = new Subject<void>();

    public pins$: Observable<PinView[]> = merge(
        this.project$.pipe(distinctUntilChanged((a, b) => a?.idProject === b?.idProject)),
        this._pinRefreshSignal$.pipe(
            withLatestFrom(this.project$),
            map(([, p]) => p)
        )
    ).pipe(
        switchMap(project =>
            this.severityStore.severitiesMapByProject$(project.idProject).pipe(
                switchMap(severities =>
                    this.sPin.loadPins(project.idProject, PinDestinationType.PROJECT).pipe(
                        map(pins =>
                            pins
                                .filter(p => !!p.issue)
                                .map(p => {
                                    const sev =
                                        p.issue?.idSeverity != null
                                            ? severities.get(p.issue.idSeverity)
                                            : undefined;
                                    return {
                                        idPin: p.idPin!,
                                        idSeverity: sev?.idSeverity,
                                        severityColor: sev?.color,
                                        severityName: sev?.title,
                                        idProject: p.issue!.idProject,
                                        idIssuePublic: p.issue!.idIssuePublic!,
                                        title: p.issue!.title,
                                        stateName: p.issue?.stateName,
                                        stateIsStart: p.issue?.stateIsStart,
                                        stateIsFinal: p.issue?.stateIsFinal,
                                        assignedToName: p.issue?.assignedToName,
                                        assignedToColorAvatarBg: p.issue?.assignedToColorAvatarBg
                                    };
                                })
                        )
                    )
                )
            )
        )
    );

    public ngOnInit(): void {
        this._loadSignal$.next();
    }

    public onDeletePin(pin: PinView): void {
        this.sPin.deletePin(pin.idPin).subscribe(() => this._pinRefreshSignal$.next());
    }
}
