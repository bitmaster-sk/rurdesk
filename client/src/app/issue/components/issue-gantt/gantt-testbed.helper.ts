import { TestBed } from '@angular/core/testing';
import { of, EMPTY } from 'rxjs';
import { Component, input, output } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { provideRouter } from '@angular/router';
import { provideNoopAnimations } from '@angular/platform-browser/animations';

import { IssueGanttComponent } from './issue-gantt.component';
import { IssueGanttService } from './service/issue-gantt.service';
import { GanttTimelineService } from './service/gantt-timeline.service';
import { GanttDragService, DragMode } from './service/gantt-drag.service';
import { GanttCascadeService } from './service/gantt-cascade.service';
import { GanttCriticalPathService, emptyCriticalPath } from './service/gantt-critical-path.service';
import { GanttZoomLevel } from './constants/gantt-zoom-config';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueBulkApi } from '../../api/issue-bulk.api.service';
import { IssueService } from '../../issue.service';
import { IssueRelationApi } from '../../api/issue-relation.api.service';
import { GanttOrderApi } from '../../api/gantt-order.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { ExtendedIssue } from '../../model/extended-issue.model';
import { ReadIssueRelationDto } from '../../model/issue-relation.model';

export function makeTask(over: Partial<ExtendedIssue> = {}): ExtendedIssue {
    return {
        idIssue: 1,
        idIssuePublic: 1,
        idProject: 10,
        title: 'Task',
        description: '',
        tracked: 0,
        idState: null,
        idSeverity: null,
        scheduledAt: new Date('2025-01-15T00:00:00Z'),
        estimated: 3600,
        state: undefined,
        severity: undefined,
        ...over
    } as ExtendedIssue;
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

@Component({ selector: 'app-gantt-wbs-panel', template: '', standalone: true })
export class WbsPanelStub {
    public readonly scheduledTasks = input.required<any[]>();
    public readonly backlogTasks = input.required<any[]>();
    public readonly backlogHasMore = input<boolean>(false);
    public readonly backlogLoading = input<boolean>(false);
    public readonly isCollapsed = input<boolean>(false);
    public readonly hoveredTaskId = input<number | null>(null);
    public readonly selectedTaskId = input<number | null>(null);
    public readonly scrolled = output<number>();
    public readonly taskClicked = output<any>();
    public readonly taskHovered = output<number | null>();
    public readonly backlogDragStarted = output<any>();
    public readonly loadMoreBacklog = output<void>();
    public readonly reordered = output<any>();
}

@Component({ selector: 'app-gantt-timeline-body', template: '', standalone: true })
export class TimelineBodyStub {
    public readonly tasks = input.required<any[]>();
    public readonly columns = input.required<any[]>();
    public readonly headerRows = input.required<any[]>();
    public readonly cardMode = input.required<any>();
    public readonly criticalTaskIds = input<any>(new Set());
    public readonly criticalRelationIds = input<any>(new Set());
    public readonly criticalRelationOrder = input<any>(new Map());
    public readonly isCriticalTracing = input<boolean>(false);
    public readonly isCriticalPathEnabled = input<boolean>(false);
    public readonly ghostBars = input<any[]>([]);
    public readonly relations = input<any[]>([]);
    public readonly selectedRelationId = input<number | null>(null);
    public readonly selectedTaskIndex = input<number | null>(null);
    public readonly drawingLine = input<any>(null);
    public readonly relationDropTarget = input<any>(null);
    public readonly drawInRelation = input<any>(null);
    public readonly cascadeSlide = input<any>(new Map());
    public readonly scrolled = output<any>();
    public readonly taskHovered = output<number | null>();
    public readonly barDragStarted = output<any>();
    public readonly barResizeEnded = output<any>();
    public readonly barContextMenu = output<any>();
    public readonly connectionDragStarted = output<any>();
    public readonly arrowClicked = output<number>();
    public readonly arrowDeleteRequested = output<any>();
    public scrollToToday = vi.fn();
    public panHorizontal = vi.fn();
    public scrollToPixel = vi.fn();
    public syncScrollFrom = vi.fn();
    public getScrollContainer = vi.fn(() => ({
        getBoundingClientRect: () => ({
            left: 0,
            right: 1000,
            top: 0,
            bottom: 500,
            width: 1000,
            height: 500
        }),
        scrollLeft: 0,
        clientWidth: 800
    }));
}

@Component({ selector: 'app-gantt-minimap', template: '', standalone: true })
export class MinimapStub {
    public readonly tasks = input<any[]>([]);
    public readonly viewportScrollLeft = input<number>(0);
    public readonly viewportWidth = input<number>(0);
    public readonly navigated = output<number>();
}

@Component({ selector: 'app-issue-quick-actions', template: '', standalone: true })
export class QuickActionsStub {
    public show = vi.fn();
}

@Component({ selector: 'app-filter', template: '', standalone: true })
export class FilterStub {}

export interface GanttMocks {
    ganttServiceMock: any;
    timelineServiceMock: any;
    dragServiceMock: any;
    cascadeServiceMock: any;
    criticalPathServiceMock: any;
    issueFilterStoreMock: any;
    commandPaletteMock: any;
    bulkApiMock: any;
    issueServiceMock: any;
    relationApiMock: any;
    ganttOrderApiMock: any;
    toastMock: any;
    noticeServiceMock: any;
}

export function configureGanttTestBed(
    overrides: {
        tasks?: ExtendedIssue[];
        relations?: ReadIssueRelationDto[];
    } = {}
): GanttMocks {
    const tasks = overrides.tasks ?? [
        makeTask({ idIssuePublic: 1 }),
        makeTask({ idIssuePublic: 2 }),
        makeTask({ idIssuePublic: 3 })
    ];
    const relations = overrides.relations ?? [];

    const ganttServiceMock = {
        data$: of({ scheduledTasks: tasks, backlogTasks: [] as ExtendedIssue[], relations }),
        backlogHasMore: vi.fn(() => false),
        backlogLoading: vi.fn(() => false),
        loadMoreBacklog: vi.fn(),
        addRelations: vi.fn(),
        removeRelation: vi.fn()
    };

    const rowHeightSignal = Object.assign(() => 72, { set: vi.fn() });

    const timelineServiceMock = {
        zoomLevel: vi.fn(() => GanttZoomLevel.Week),
        setZoom: vi.fn(),
        rowHeight: rowHeightSignal,
        getColumns: vi.fn(() => []),
        getHeaderRows: vi.fn(() => []),
        getTotalWidth: vi.fn(() => 1000),
        computeRange: vi.fn(() => ({ start: new Date('2025-01-01'), end: new Date('2025-02-01') })),
        setRange: vi.fn(),
        toPixel: vi.fn(() => 100),
        toDate: vi.fn(() => new Date('2025-01-15'))
    };

    const dragServiceMock = {
        state: vi.fn(() => ({
            mode: DragMode.Idle,
            taskId: null,
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        })),
        completed: vi.fn(() => null),
        reset: vi.fn(),
        cancel: vi.fn(),
        isDragging: vi.fn(() => false),
        getMoveDelta: vi.fn(() => null),
        getBacklogScheduleResult: vi.fn(() => null),
        lastDropTarget: vi.fn(() => null),
        activeDropTarget: vi.fn(() => null),
        startMove: vi.fn(),
        startBacklogSchedule: vi.fn(),
        startRelationDraw: vi.fn(),
        setCanvasOffset: vi.fn(),
        tooltipText: vi.fn(() => null)
    };

    const cascadeServiceMock = {
        computeCascade: vi.fn(() => ({ affectedTasks: new Map() }))
    };

    const criticalPathServiceMock = {
        computeCriticalPath: vi.fn(() => emptyCriticalPath())
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

    const commandPaletteMock = {
        isOverlayOpen: vi.fn(() => false),
        setContext: vi.fn()
    };

    const bulkApiMock = { update$: vi.fn(() => mockSub()) };
    const issueServiceMock = { updateIssue: vi.fn(() => mockSub()) };
    const relationApiMock = { insert$: vi.fn(() => mockSub()), delete$: vi.fn(() => mockSub()) };
    const ganttOrderApiMock = { reorder$: vi.fn(() => mockSub()) };
    const toastMock = { showError: vi.fn() };
    const noticeServiceMock = { show: vi.fn(), relation$: EMPTY, issue$: EMPTY };

    TestBed.configureTestingModule({
        imports: [
            TranslateModule.forRoot(),
            WbsPanelStub,
            TimelineBodyStub,
            MinimapStub,
            QuickActionsStub,
            FilterStub
        ],
        declarations: [IssueGanttComponent],
        providers: [
            { provide: IssueGanttService, useValue: ganttServiceMock },
            { provide: GanttTimelineService, useValue: timelineServiceMock },
            { provide: GanttDragService, useValue: dragServiceMock },
            { provide: GanttCascadeService, useValue: cascadeServiceMock },
            { provide: GanttCriticalPathService, useValue: criticalPathServiceMock },
            { provide: IssueFilterStore, useValue: issueFilterStoreMock },
            { provide: ProjectStore, useValue: projectStoreMock },
            { provide: IssueBulkApi, useValue: bulkApiMock },
            { provide: IssueService, useValue: issueServiceMock },
            { provide: IssueRelationApi, useValue: relationApiMock },
            { provide: GanttOrderApi, useValue: ganttOrderApiMock },
            { provide: NoticeService, useValue: noticeServiceMock },
            { provide: IssueToolbarService, useValue: { register: vi.fn(), clear: vi.fn() } },
            { provide: ToastNotificationService, useValue: toastMock },
            { provide: CommandPaletteService, useValue: commandPaletteMock },
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

    TestBed.overrideComponent(IssueGanttComponent, {
        set: {
            providers: [
                { provide: IssueGanttService, useValue: ganttServiceMock },
                { provide: GanttTimelineService, useValue: timelineServiceMock },
                { provide: GanttDragService, useValue: dragServiceMock },
                { provide: GanttCascadeService, useValue: cascadeServiceMock },
                { provide: GanttCriticalPathService, useValue: criticalPathServiceMock }
            ]
        }
    });

    return {
        ganttServiceMock,
        timelineServiceMock,
        dragServiceMock,
        cascadeServiceMock,
        criticalPathServiceMock,
        issueFilterStoreMock,
        commandPaletteMock,
        bulkApiMock,
        issueServiceMock,
        relationApiMock,
        ganttOrderApiMock,
        toastMock,
        noticeServiceMock
    };
}

export async function createGanttFixture(
    overrides: {
        tasks?: ExtendedIssue[];
        relations?: ReadIssueRelationDto[];
    } = {}
): Promise<{ fixture: any; comp: IssueGanttComponent; mocks: GanttMocks }> {
    const mocks = configureGanttTestBed(overrides);
    await TestBed.compileComponents();
    const fixture = TestBed.createComponent(IssueGanttComponent);
    const comp = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, comp, mocks };
}
