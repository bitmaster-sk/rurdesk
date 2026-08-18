import { Injector, runInInjectionContext } from '@angular/core';
import { EventInput } from '@fullcalendar/core';
import { of } from 'rxjs';
import { IssueCalendarService } from './issue-calendar.service';
import { IssueService } from '../../../issue.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { StateStore } from 'src/app/state/store/state.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { Issue } from '../../../model/issue.model';

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
        ...over
    };
}

function buildService(issues: Issue[]): IssueCalendarService {
    const injector = Injector.create({
        providers: [
            { provide: IssueService, useValue: { loadIssues: () => of(issues) } },
            { provide: SeverityStore, useValue: { severitiesMapByProject$: () => of(new Map()) } },
            { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
            { provide: StateStore, useValue: { statesMapByProject$: () => of(new Map()) } },
            {
                provide: IssueFilterStore,
                useValue: { clear: () => {}, actualFilter$: of({ idProject: 1 }) }
            }
        ]
    });
    return runInInjectionContext(injector, () => new IssueCalendarService());
}

function firstEmit(svc: IssueCalendarService): EventInput[] {
    let value: EventInput[] = [];
    svc.events$.subscribe(v => (value = v));
    return value;
}

describe('IssueCalendarService — events$', () => {
    it('excludes issues without a scheduledAt', () => {
        const events = firstEmit(buildService([makeIssue({ idIssue: 1, scheduledAt: undefined })]));
        expect(events).toHaveLength(0);
    });

    it('maps a scheduled+estimated issue to a timed event with computed end', () => {
        const events = firstEmit(
            buildService([
                makeIssue({
                    idIssue: 7,
                    scheduledAt: new Date('2026-04-10T09:00:00Z'),
                    estimated: 3600
                })
            ])
        );
        expect(events).toHaveLength(1);
        expect(events[0].id).toBe('7');
        expect(events[0].allDay).toBe(false);
        expect(events[0].start).toBe('2026-04-10T09:00:00.000Z');
        expect(events[0].end).toBe('2026-04-10T10:00:00.000Z');
    });

    it('marks an issue with no estimate as all-day with no end', () => {
        const events = firstEmit(
            buildService([
                makeIssue({
                    idIssue: 8,
                    scheduledAt: new Date('2026-04-10T09:00:00Z'),
                    estimated: 0
                })
            ])
        );
        expect(events[0].allDay).toBe(true);
        expect(events[0].end).toBeUndefined();
    });
});
