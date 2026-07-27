import { TestBed } from '@angular/core/testing';
import { of, EMPTY } from 'rxjs';
import { Component, input, output } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { FormsModule } from '@angular/forms';
import { provideRouter } from '@angular/router';
import { provideNoopAnimations } from '@angular/platform-browser/animations';

import { IssueTableComponent } from './issue-table.component';
import { IssueTableService } from './service/issue-table.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { ProjectStore } from 'src/app/project/project.store';
import { StateStore } from 'src/app/state/store/state.store';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { HotkeyService } from 'src/app/core/command/hotkey.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { Issue } from '../../model/issue.model';
import { UiModule } from 'src/app/ui/ui.module';

import { signal } from '@angular/core';

export function makeIssue(over: Partial<Issue> = {}): Issue {
    return { idIssuePublic: 1, idProject: 5, title: 'T', tracked: 0, ...over } as Issue;
}

export function mockSub<T = unknown>() {
    const handlers: { next?: (v: T) => void; error?: (e: unknown) => void } = {};
    const subscribe = vi.fn((obs: any) => {
        handlers.next = obs.next;
        handlers.error = obs.error;
        return { unsubscribe: vi.fn() };
    });
    return { subscribe, handlers };
}

@Component({ selector: 'tabler-icon', template: '', standalone: true })
export class TablerIconStub {
    public readonly icon = input<string>('');
    public readonly size = input<unknown>(undefined);
}

@Component({ selector: 'app-filter', template: '', standalone: true })
export class FilterStub {}

@Component({ selector: 'app-issue-quick-actions', template: '', standalone: true })
export class QuickActionsStub {
    public readonly splitRequested = output<Issue>();
    public show = vi.fn();
}

@Component({ selector: 'app-severity-circle', template: '', standalone: true })
export class SeverityCircleStub {
    public readonly color = input<string | null>(null);
}

@Component({ selector: 'app-state-badge', template: '', standalone: true })
export class StateBadgeStub {
    public readonly state = input<any>(undefined);
    public readonly size = input<string>('s');
}

@Component({ selector: 'app-avatar', template: '', standalone: true })
export class AvatarStub {
    public readonly name = input<string>('');
    public readonly bgColor = input<string>('');
    public readonly height = input<number>(0);
    public readonly width = input<number>(0);
}

@Component({ selector: 'app-quality-badge', template: '', standalone: true })
export class QualityBadgeStub {
    public readonly score = input<number | null>(null);
}

@Component({ selector: 'app-issue-table-drop-zone', template: '', standalone: true })
export class DropZoneStub {
    public readonly targetIssue = input<any>(null);
    public readonly relationDrop = output<any>();
    public readonly zoneEnter = output<void>();
}

@Component({ selector: 'app-split-dialog', template: '', standalone: true })
export class SplitDialogStub {
    public readonly idProject = input<number>(0);
    public readonly issue = input<any>(null);
    public readonly accepted = output<any>();
    public readonly cancelled = output<void>();
}

export interface TableMocks {
    issueTableServiceMock: any;
    issueFilterStoreMock: any;
    projectStoreMock: any;
    stateStoreMock: any;
    commandPaletteMock: any;
    hotkeysMock: any;
}

export function configureTableTestBed(): TableMocks {
    const issueTableServiceMock = {
        rows: signal<any[]>([]),
        total: signal(0),
        isLoading: signal(false),
        canLoadMore: vi.fn(() => false),
        insertRelation$: vi.fn(() => ({ pipe: vi.fn(() => ({ subscribe: vi.fn() })) })),
        loadRelationsFor: vi.fn()
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

    const stateStoreMock = {
        statesByProject$: vi.fn(() => of([]))
    };

    const commandPaletteMock = {
        isOverlayOpen: vi.fn(() => false),
        setContext: vi.fn()
    };

    const hotkeysMock = {
        registerListHandler: vi.fn()
    };

    TestBed.configureTestingModule({
        imports: [
            TranslateModule.forRoot(),
            FormsModule,
            UiModule,
            TablerIconStub,
            FilterStub,
            QuickActionsStub,
            SeverityCircleStub,
            StateBadgeStub,
            AvatarStub,
            QualityBadgeStub,
            DropZoneStub,
            SplitDialogStub
        ],
        declarations: [IssueTableComponent],
        providers: [
            { provide: IssueTableService, useValue: issueTableServiceMock },
            { provide: IssueFilterStore, useValue: issueFilterStoreMock },
            { provide: ProjectStore, useValue: projectStoreMock },
            { provide: StateStore, useValue: stateStoreMock },
            { provide: IssueToolbarService, useValue: { register: vi.fn(), clear: vi.fn() } },
            { provide: HotkeyService, useValue: hotkeysMock },
            { provide: CommandPaletteService, useValue: commandPaletteMock },
            {
                provide: NoticeService,
                useValue: { show: vi.fn(), relation$: EMPTY, issue$: EMPTY }
            },
            { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
            provideRouter([]),
            provideNoopAnimations()
        ]
    });

    TestBed.overrideComponent(IssueTableComponent, {
        set: {
            providers: [{ provide: IssueTableService, useValue: issueTableServiceMock }]
        }
    });

    return {
        issueTableServiceMock,
        issueFilterStoreMock,
        projectStoreMock,
        stateStoreMock,
        commandPaletteMock,
        hotkeysMock
    };
}

export async function createTableFixture(): Promise<{
    fixture: any;
    comp: IssueTableComponent;
    mocks: TableMocks;
}> {
    const mocks = configureTableTestBed();
    await TestBed.compileComponents();
    const fixture = TestBed.createComponent(IssueTableComponent);
    const comp = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, comp, mocks };
}
