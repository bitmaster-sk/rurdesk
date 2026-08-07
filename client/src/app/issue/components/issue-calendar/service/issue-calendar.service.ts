import { Injectable, inject } from '@angular/core';
import { EventInput } from '@fullcalendar/core';
import { add } from 'date-fns';
import { map, switchMap, withLatestFrom } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssueService } from '../../../issue.service';
import { Issue } from '../../../model/issue.model';

@Injectable()
export class IssueCalendarService {
    private readonly issueService = inject(IssueService);
    private readonly severityStore = inject(SeverityStore);
    private readonly memberStore = inject(ProjectMemberStore);
    private readonly stateStore = inject(StateStore);
    private readonly issueFilterStore = inject(IssueFilterStore);

    constructor() {
        // Drop any leftover filter from a previously-mounted view so we don't fire a
        // stale load before this view's setInitialFilter runs.
        this.issueFilterStore.clear();
    }

    public readonly events$ = this.issueFilterStore.actualFilter$.pipe(
        switchMap(filter =>
            this.issueService
                .loadIssues(filter)
                .pipe(
                    withLatestFrom(
                        this.severityStore.severitiesMapByProject$(filter.idProject),
                        this.memberStore.usersMap$,
                        this.stateStore.statesMapByProject$(filter.idProject)
                    )
                )
        ),
        map(([issues, severities, users, states]) =>
            this.toCalendarEvents(issues, severities, users, states)
        )
    );

    private toCalendarEvents(
        issues: Issue[],
        severities: Map<number, IssueSeverity>,
        users: Map<number, User>,
        states: Map<number, IssueState>
    ): EventInput[] {
        return issues
            .filter((issue): issue is Issue & { scheduledAt: Date } => !!issue.scheduledAt)
            .map(issue => {
                const severity =
                    issue.idSeverity !== null ? severities.get(issue.idSeverity) : undefined;
                const assigned = issue.assignedTo != null ? users.get(issue.assignedTo) : undefined;
                const state = issue.idState !== null ? states.get(issue.idState) : undefined;
                const isAllDay = !issue.estimated;
                const severityColor = severity?.color ?? null;
                return {
                    id: `${issue.idIssue}`,
                    allDay: isAllDay,
                    title: issue.title,
                    start: issue.scheduledAt.toISOString(),
                    end: isAllDay
                        ? undefined
                        : add(issue.scheduledAt, { seconds: issue.estimated ?? 0 }).toISOString(),
                    extendedProps: { severity, assigned, state, issue }
                };
            });
    }
}
