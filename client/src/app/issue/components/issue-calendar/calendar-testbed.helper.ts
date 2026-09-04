import { TestBed } from '@angular/core/testing';
import { of, EMPTY } from 'rxjs';
import { Component } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { provideRouter } from '@angular/router';
import { provideNoopAnimations } from '@angular/platform-browser/animations';

import { IssueCalendarComponent } from './issue-calendar.component';
import { IssueCalendarService } from './service/issue-calendar.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueService } from '../../issue.service';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';

export function mockSub<T = unknown>() {
    const handlers: { next?: (v: T) => void; error?: (e: unknown) => void } = {};
    const subscribe = vi.fn((obs: any) => {
        handlers.next = obs.next;
        handlers.error = obs.error;
        return { unsubscribe: vi.fn() };
    });
    return { subscribe, handlers };
}

function makeCalendarApi() {
    const el = document.createElement('div');
    const api = {
        today: vi.fn(),
        prev: vi.fn(),
        next: vi.fn(),
        changeView: vi.fn(),
        render: vi.fn(),
        updateSize: vi.fn(),
        removeAllEventSources: vi.fn(),
        addEventSource: vi.fn(),
        setOption: vi.fn(),
        on: vi.fn(),
        el
    };
    return api;
}

@Component({ selector: 'full-calendar', template: '', standalone: true })
export class FullCalendarStub {
    private readonly _api = makeCalendarApi();
    public getApi = vi.fn(() => this._api);
}

@Component({ selector: 'app-issue-quick-actions', template: '', standalone: true })
export class QuickActionsStub {
    public show = vi.fn();
}

@Component({ selector: 'app-filter', template: '', standalone: true })
export class FilterStub {}

export interface CalendarMocks {
    issueCalendarServiceMock: any;
    issueFilterStoreMock: any;
    projectStoreMock: any;
    sIssueMock: any;
    commandPaletteMock: any;
    noticeServiceMock: any;
}

export function configureCalendarTestBed(
    overrides: {
        events$?: any;
    } = {}
): CalendarMocks {
    const events$ = overrides.events$ ?? of([]);

    const issueCalendarServiceMock = {
        events$
    };

    const issueFilterStoreMock = {
        showFilter$: of(false),
        toggleShowFilter: vi.fn(),
        setFilter: vi.fn(),
        setInitialFilter: vi.fn(),
        refresh: vi.fn(),
        clear: vi.fn(),
        actualFilter$: of({ idProject: 10 }),
        actualFilterChange$: of({ filter: { idProject: 10 }, refresh: false })
    };

    const projectStoreMock = {
        project$: of({ idProject: 10 })
    };

    const sIssueMock = {
        updateIssue: vi.fn(() => mockSub())
    };

    const commandPaletteMock = {
        isOverlayOpen: vi.fn(() => false),
        setContext: vi.fn()
    };

    const noticeServiceMock = {
        show: vi.fn(),
        relation$: EMPTY,
        issue$: EMPTY
    };

    TestBed.configureTestingModule({
        imports: [TranslateModule.forRoot(), FullCalendarStub, QuickActionsStub, FilterStub],
        declarations: [IssueCalendarComponent],
        providers: [
            { provide: IssueCalendarService, useValue: issueCalendarServiceMock },
            { provide: IssueFilterStore, useValue: issueFilterStoreMock },
            { provide: ProjectStore, useValue: projectStoreMock },
            { provide: IssueService, useValue: sIssueMock },
            { provide: IssueToolbarService, useValue: { register: vi.fn(), clear: vi.fn() } },
            { provide: NoticeService, useValue: noticeServiceMock },
            { provide: CommandPaletteService, useValue: commandPaletteMock },
            { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
            {
                provide: SeverityStore,
                useValue: { severitiesByProject$: vi.fn(() => of(new Map())) }
            },
            { provide: StateStore, useValue: { statesByProject$: vi.fn(() => of(new Map())) } },
            { provide: ProjectMemberStore, useValue: { users$: of([]), usersMap$: of(new Map()) } },
            provideRouter([]),
            provideNoopAnimations()
        ]
    });

    TestBed.overrideComponent(IssueCalendarComponent, {
        set: {
            providers: [{ provide: IssueCalendarService, useValue: issueCalendarServiceMock }]
        }
    });

    return {
        issueCalendarServiceMock,
        issueFilterStoreMock,
        projectStoreMock,
        sIssueMock,
        commandPaletteMock,
        noticeServiceMock
    };
}

export async function createCalendarFixture(
    overrides: {
        events$?: any;
    } = {}
): Promise<{ fixture: any; comp: IssueCalendarComponent; mocks: CalendarMocks }> {
    const mocks = configureCalendarTestBed(overrides);
    await TestBed.compileComponents();
    const fixture = TestBed.createComponent(IssueCalendarComponent);
    const comp = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, comp, mocks };
}
