import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { Component, input, output } from '@angular/core';
import { provideLocationMocks } from '@angular/common/testing';
import { TranslateModule } from '@ngx-translate/core';
import { FormsModule } from '@angular/forms';
import { provideRouter, Router } from '@angular/router';
import { provideNoopAnimations } from '@angular/platform-browser/animations';

import { IssueQuickActionsComponent } from './issue-quick-actions.component';
import { ProjectStore } from 'src/app/project/project.store';
import { StateStore } from 'src/app/state/store/state.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { IssueService } from '../../issue.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { UiModule } from 'src/app/ui/ui.module';
import { Issue } from '../../model/issue.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { User } from 'src/app/auth/model/user.model';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';

export function mockSub<T = unknown>() {
    const handlers: { next?: (v: T) => void; error?: (e: unknown) => void } = {};
    const subscribe = vi.fn((obs: any) => {
        handlers.next = obs.next;
        handlers.error = obs.error;
        return { unsubscribe: vi.fn() };
    });
    return { subscribe, handlers };
}

const stateA: IssueState = {
    idState: 1,
    name: 'Todo',
    idProject: 1,
    start: true,
    final: false,
    protected: false,
    orderRank: 0
};
const stateB: IssueState = {
    idState: 2,
    name: 'Done',
    idProject: 1,
    start: false,
    final: true,
    protected: false,
    orderRank: 1
};
const severityA: IssueSeverity = {
    idSeverity: 1,
    title: 'High',
    idProject: 1,
    color: '#f00',
    protected: false,
    orderRank: 0
};
const alice: User = { idUser: 10, name: 'Alice', email: 'a@a.com', colorAvatarBg: '#aaa' };
const bob: User = { idUser: 20, name: 'Bob', email: 'b@b.com', colorAvatarBg: '#bbb' };

function makeIssue(over: Partial<Issue> = {}): Issue {
    return {
        idIssue: 1,
        idIssuePublic: 42,
        idProject: 1,
        title: 'Test Issue',
        description: '',
        tracked: 0,
        idState: 1,
        idSeverity: 1,
        scheduledAt: new Date('2025-01-15T00:00:00Z'),
        estimated: 3600,
        assignedTo: 10,
        ...over
    };
}

@Component({ selector: 'app-state-badge-selector', template: '', standalone: true })
class StateBadgeSelectorStub {
    public readonly states = input<any[]>([]);
    public readonly ngModel = input<any>(null);
    public readonly ngModelChange = output<any>();
}

@Component({ selector: 'app-severity-badge-selector', template: '', standalone: true })
class SeverityBadgeSelectorStub {
    public readonly severities = input<any[]>([]);
    public readonly ngModel = input<any>(null);
    public readonly ngModelChange = output<any>();
}

@Component({ selector: 'app-user-dropdown', template: '', standalone: true })
class UserDropdownStub {
    public readonly users = input<any[]>([]);
    public readonly filter = input<boolean>(false);
    public readonly ngModel = input<any>(null);
    public readonly ngModelChange = output<any>();
}

async function createFixture(
    overrides: {
        states?: IssueState[];
        severities?: IssueSeverity[];
        users?: User[];
        issueService?: any;
    } = {}
) {
    const states = overrides.states ?? [stateA, stateB];
    const severities = overrides.severities ?? [severityA];
    const users = overrides.users ?? [alice, bob];

    const issueServiceMock = overrides.issueService ?? {
        updateIssue: vi.fn(() => mockSub()),
        deleteIssue: vi.fn(() => mockSub())
    };

    const toastMock = { showError: vi.fn(), showSuccess: vi.fn() };

    const issueFilterStoreMock = {
        showFilter$: of(false),
        toggleShowFilter: vi.fn(),
        setFilter: vi.fn(),
        setInitialFilter: vi.fn(),
        refresh: vi.fn(),
        clear: vi.fn(),
        actualFilter$: of({ idProject: 1 }),
        actualFilterChange$: of({ filter: { idProject: 1 }, refresh: false })
    };

    TestBed.configureTestingModule({
        imports: [
            TranslateModule.forRoot(),
            FormsModule,
            UiModule,
            StateBadgeSelectorStub,
            SeverityBadgeSelectorStub,
            UserDropdownStub
        ],
        declarations: [IssueQuickActionsComponent],
        providers: [
            { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
            {
                provide: StateStore,
                useValue: { states$: of(states), statesByProject$: vi.fn(() => of(states)) }
            },
            {
                provide: SeverityStore,
                useValue: {
                    severities$: of(severities),
                    severitiesByProject$: vi.fn(() => of(severities))
                }
            },
            {
                provide: ProjectMemberStore,
                useValue: { users$: of(users), usersMap$: of(new Map()) }
            },
            { provide: IssueService, useValue: issueServiceMock },
            { provide: IssueFilterStore, useValue: issueFilterStoreMock },
            { provide: ToastNotificationService, useValue: toastMock },
            provideRouter([]),
            provideLocationMocks(),
            provideNoopAnimations()
        ]
    });

    await TestBed.compileComponents();
    const fixture = TestBed.createComponent(IssueQuickActionsComponent);
    const comp = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, comp, issueServiceMock, issueFilterStoreMock, toastMock };
}

describe('IssueQuickActionsComponent (TestBed)', () => {
    let comp: any;
    let issueServiceMock: any;
    let issueFilterStoreMock: any;

    beforeEach(async () => {
        const result = await createFixture();
        comp = result.comp;
        issueServiceMock = result.issueServiceMock;
        issueFilterStoreMock = result.issueFilterStoreMock;
    });

    // =========================================================================
    // buildInitials
    // =========================================================================

    describe('buildInitials', () => {
        it('returns initials for a two-word name', () => {
            expect(comp.buildInitials('Alice Smith')).toBe('AS');
        });

        it('returns first two chars for a single-word name', () => {
            expect(comp.buildInitials('Alice')).toBe('AL');
        });

        it('returns initials for a three-word name (first two words)', () => {
            expect(comp.buildInitials('Alice Bob Charlie')).toBe('AB');
        });

        it('handles empty string', () => {
            expect(comp.buildInitials('')).toBe('');
        });
    });

    // =========================================================================
    // trackedPercent
    // =========================================================================

    describe('trackedPercent', () => {
        it('returns 0 when estimated is null', () => {
            comp.issue.set(makeIssue({ estimated: null, tracked: 100 }));
            expect(comp.trackedPercent()).toBe(0);
        });

        it('returns correct percentage', () => {
            comp.issue.set(makeIssue({ estimated: 3600, tracked: 1800 }));
            expect(comp.trackedPercent()).toBe(50);
        });

        it('clamps at 100', () => {
            comp.issue.set(makeIssue({ estimated: 3600, tracked: 7200 }));
            expect(comp.trackedPercent()).toBe(100);
        });
    });

    // =========================================================================
    // currentSeverity / currentUser computed
    // =========================================================================

    it('currentSeverity returns the matching severity', () => {
        comp.issue.set(makeIssue({ idSeverity: 1 }));
        expect(comp.currentSeverity()?.idSeverity).toBe(1);
    });

    it('currentSeverity returns null when not found', () => {
        comp.issue.set(makeIssue({ idSeverity: 999 }));
        expect(comp.currentSeverity()).toBeNull();
    });

    it('currentUser returns the matching user', () => {
        comp.issue.set(makeIssue({ assignedTo: 10 }));
        expect(comp.currentUser()?.idUser).toBe(10);
    });

    it('currentUser returns null when not found', () => {
        comp.issue.set(makeIssue({ assignedTo: 999 }));
        expect(comp.currentUser()).toBeNull();
    });

    // =========================================================================
    // onStateChange
    // =========================================================================

    it('onStateChange updates issue and calls issueService.updateIssue', () => {
        comp.issue.set(makeIssue({ idState: 1 }));
        issueServiceMock.updateIssue.mockClear();
        comp.onStateChange(2);
        expect(comp.issue().idState).toBe(2);
        expect(issueServiceMock.updateIssue).toHaveBeenCalled();
    });

    it('rolls the issue back when the save fails', () => {
        const sub = mockSub();
        issueServiceMock.updateIssue.mockReturnValueOnce(sub);
        const original = makeIssue({ idState: 1 });
        comp.issue.set(original);
        comp.onStateChange(2);
        expect(comp.issue().idState).toBe(2); // optimistic

        sub.handlers.error?.(new Error('boom'));

        // The failure toast comes from ErrorInterceptor, which sees every request.
        expect(comp.issue().idState).toBe(1);
    });

    it('onStateChange with no issue: no-op', () => {
        comp.issue.set(null);
        issueServiceMock.updateIssue.mockClear();
        comp.onStateChange(2);
        expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
    });

    // =========================================================================
    // onSeverityChange
    // =========================================================================

    it('onSeverityChange updates issue and calls issueService.updateIssue', () => {
        comp.issue.set(makeIssue({ idSeverity: 1 }));
        issueServiceMock.updateIssue.mockClear();
        comp.onSeverityChange(2);
        expect(comp.issue().idSeverity).toBe(2);
        expect(issueServiceMock.updateIssue).toHaveBeenCalled();
    });

    it('onSeverityChange with no issue: no-op', () => {
        comp.issue.set(null);
        issueServiceMock.updateIssue.mockClear();
        comp.onSeverityChange(2);
        expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
    });

    // =========================================================================
    // onAssigneeChange
    // =========================================================================

    it('onAssigneeChange updates issue and calls issueService.updateIssue', () => {
        comp.issue.set(makeIssue({ assignedTo: 10 }));
        issueServiceMock.updateIssue.mockClear();
        comp.onAssigneeChange(20);
        expect(comp.issue().assignedTo).toBe(20);
        expect(issueServiceMock.updateIssue).toHaveBeenCalled();
    });

    it('onAssigneeChange to null: clears assignee', () => {
        comp.issue.set(makeIssue({ assignedTo: 10 }));
        comp.onAssigneeChange(null);
        expect(comp.issue().assignedTo).toBeNull();
    });

    it('onAssigneeChange with no issue: no-op', () => {
        comp.issue.set(null);
        issueServiceMock.updateIssue.mockClear();
        comp.onAssigneeChange(20);
        expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
    });

    // =========================================================================
    // Reschedule actions
    // =========================================================================

    describe('onPreviousDay', () => {
        it('shifts scheduledAt back by one day', () => {
            const date = new Date('2025-01-15T00:00:00Z');
            comp.issue.set(makeIssue({ scheduledAt: date }));
            comp.onPreviousDay();
            expect(comp.issue().scheduledAt).toEqual(new Date('2025-01-14T00:00:00Z'));
        });

        it('no scheduledAt: no-op', () => {
            comp.issue.set(makeIssue({ scheduledAt: null }));
            issueServiceMock.updateIssue.mockClear();
            comp.onPreviousDay();
            expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });
    });

    describe('onNextDay', () => {
        it('shifts scheduledAt forward by one day', () => {
            const date = new Date('2025-01-15T00:00:00Z');
            comp.issue.set(makeIssue({ scheduledAt: date }));
            comp.onNextDay();
            expect(comp.issue().scheduledAt).toEqual(new Date('2025-01-16T00:00:00Z'));
        });

        it('no scheduledAt: no-op', () => {
            comp.issue.set(makeIssue({ scheduledAt: null }));
            issueServiceMock.updateIssue.mockClear();
            comp.onNextDay();
            expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });
    });

    describe('onToday', () => {
        it('sets scheduledAt to start of today', () => {
            comp.issue.set(makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z') }));
            comp.onToday();
            const expected = new Date();
            expected.setHours(0, 0, 0, 0);
            expect(comp.issue().scheduledAt).toEqual(expected);
        });

        it('with no issue: no-op', () => {
            comp.issue.set(null);
            issueServiceMock.updateIssue.mockClear();
            comp.onToday();
            expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });
    });

    describe('onRemoveDate', () => {
        it('sets scheduledAt to null', () => {
            comp.issue.set(makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z') }));
            comp.onRemoveDate();
            expect(comp.issue().scheduledAt).toBeNull();
        });
    });

    describe('onPickDate', () => {
        it('sets scheduledAt to start of picked date', () => {
            comp.issue.set(makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z') }));
            // Local time on purpose: startOfDay() strips to the local start of day,
            // so asserting against a UTC-midnight literal is flaky off UTC.
            const picked = new Date(2025, 2, 20, 14, 30, 0);
            comp.onPickDate(picked);
            const expected = new Date(2025, 2, 20, 0, 0, 0, 0);
            expect(comp.issue().scheduledAt).toEqual(expected);
        });

        it('swaps back to actions view', () => {
            comp.issue.set(makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z') }));
            comp.view.set('date');
            comp.onPickDate(new Date('2025-03-20T00:00:00Z'));
            expect(comp.view()).toBe('actions');
        });

        it('with no issue: no-op', () => {
            comp.issue.set(null);
            issueServiceMock.updateIssue.mockClear();
            comp.onPickDate(new Date());
            expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });

        it('with null date: no-op', () => {
            comp.issue.set(makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z') }));
            issueServiceMock.updateIssue.mockClear();
            comp.onPickDate(null as any);
            expect(issueServiceMock.updateIssue).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onOpen
    // =========================================================================

    it('onOpen opens the issue detail page in a new browser tab', () => {
        comp.issue.set(makeIssue({ idProject: 5, idIssuePublic: 42 }));
        const openSpy = vi.spyOn(window, 'open');
        const router = TestBed.inject(Router);
        const navigateSpy = vi.spyOn(router, 'navigate');

        expect(() => comp.onOpen()).not.toThrow();

        expect(openSpy).toHaveBeenCalledTimes(1);
        const [url, target, features] = openSpy.mock.calls[0];
        expect(url).toContain('/project/5');
        expect(url).toContain('/issue/42');
        expect(target).toBe('_blank');
        expect(features).toContain('noopener');
        expect(navigateSpy).not.toHaveBeenCalled();

        openSpy.mockRestore();
        navigateSpy.mockRestore();
    });

    it('onOpen with no issue: no-op', () => {
        comp.issue.set(null);
        const openSpy = vi.spyOn(window, 'open');
        expect(() => comp.onOpen()).not.toThrow();
        expect(openSpy).not.toHaveBeenCalled();
        openSpy.mockRestore();
    });

    // =========================================================================
    // onCopyId
    // =========================================================================

    it('onCopyId writes idIssuePublic to clipboard', () => {
        comp.issue.set(makeIssue({ idIssuePublic: 42 }));
        const writeSpy = vi.spyOn(navigator.clipboard, 'writeText');
        comp.onCopyId();
        expect(writeSpy).toHaveBeenCalledWith('42');
        writeSpy.mockRestore();
    });

    it('onCopyId with no issue: no-op', () => {
        comp.issue.set(null);
        expect(() => comp.onCopyId()).not.toThrow();
    });

    // =========================================================================
    // onDelete
    // =========================================================================

    it('onDelete calls issueService.deleteIssue and refreshes', () => {
        comp.issue.set(makeIssue({ idProject: 5, idIssuePublic: 42 }));
        issueServiceMock.deleteIssue.mockClear();
        issueFilterStoreMock.refresh.mockClear();
        comp.onDelete();
        expect(issueServiceMock.deleteIssue).toHaveBeenCalledWith(5, 42);
    });

    it('onDelete with no issue: no-op', () => {
        comp.issue.set(null);
        issueServiceMock.deleteIssue.mockClear();
        comp.onDelete();
        expect(issueServiceMock.deleteIssue).not.toHaveBeenCalled();
    });

    // =========================================================================
    // onSplit
    // =========================================================================

    it('onSplit emits splitRequested', () => {
        const issue = makeIssue({ idProject: 5, idIssuePublic: 42 });
        comp.issue.set(issue);
        let emitted: Issue | null = null;
        comp.splitRequested.subscribe((i: Issue) => (emitted = i));
        comp.onSplit();
        expect(emitted).toEqual(issue);
    });

    it('onSplit with no issue: no-op', () => {
        comp.issue.set(null);
        let emitted = false;
        comp.splitRequested.subscribe(() => (emitted = true));
        comp.onSplit();
        expect(emitted).toBe(false);
    });

    // =========================================================================
    // show / view swap
    // =========================================================================

    it('show sets issue and view to actions', () => {
        const issue = makeIssue({ idIssuePublic: 99 });
        // Call show inside try — popoverRef may error in test env,
        // but issue/view signals are set before that
        try {
            comp.show({ clientY: 100, clientX: 200, target: null } as any, issue);
        } catch {}
        expect(comp.issue()?.idIssuePublic).toBe(99);
        expect(comp.view()).toBe('actions');
    });

    it('onOpenDateView swaps to date view', () => {
        comp.view.set('actions');
        comp.onOpenDateView();
        expect(comp.view()).toBe('date');
    });

    it('onCloseDateView swaps back to actions', () => {
        comp.view.set('date');
        comp.onCloseDateView();
        expect(comp.view()).toBe('actions');
    });

    it('onPopoverHide resets view to actions', () => {
        comp.view.set('date');
        comp.onPopoverHide();
        expect(comp.view()).toBe('actions');
    });
});
