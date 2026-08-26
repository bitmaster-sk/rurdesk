import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ReactiveFormsModule, NG_VALUE_ACCESSOR, ControlValueAccessor } from '@angular/forms';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { BehaviorSubject, of, throwError } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { Component, Input, forwardRef } from '@angular/core';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { WorkflowEventStateMapComponent } from './workflow-event-state-map.component';
import { WorkflowEventMapApi } from '../../api/workflow-event-map.api.service';
import { StateStore } from '../../../state/store/state.store';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { UiLoaderStub } from 'src/testing/stubs';

// Minimal CVA stub for the state dropdown so the reactive form control binds
// without pulling in ui-select's overlay dependency tree.
@Component({
    selector: 'app-state-dropdown',
    template: '',
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateDropdownStub),
            multi: true
        }
    ]
})
class StateDropdownStub implements ControlValueAccessor {
    @Input() public states: unknown;
    @Input() public placeholder: unknown;
    @Input() public saveStatus: unknown;
    public writeValue(): void {}
    public registerOnChange(): void {}
    public registerOnTouched(): void {}
}

describe('WorkflowEventStateMapComponent', () => {
    let component: WorkflowEventStateMapComponent;
    let fixture: ComponentFixture<WorkflowEventStateMapComponent>;
    let workflowEventMapApi: any;
    let stateStore: any;
    let toast: any;

    const mockStates = [
        {
            idState: 1,
            idProject: 10,
            name: 'To Do',
            start: true,
            final: false,
            protected: false,
            orderRank: 1
        },
        {
            idState: 2,
            idProject: 10,
            name: 'In Progress',
            start: false,
            final: false,
            protected: false,
            orderRank: 2
        },
        {
            idState: 3,
            idProject: 10,
            name: 'Done',
            start: false,
            final: true,
            protected: false,
            orderRank: 3
        }
    ];

    beforeEach(async () => {
        workflowEventMapApi = { load$: vi.fn(), replace$: vi.fn() };
        stateStore = { statesByProject$: vi.fn() };
        toast = { showSuccess: vi.fn(), showError: vi.fn() };

        workflowEventMapApi.load$.mockReturnValue(of([]));
        stateStore.statesByProject$.mockReturnValue(of(mockStates));
        workflowEventMapApi.replace$.mockReturnValue(of([]));

        TestBed.configureTestingModule({
            declarations: [WorkflowEventStateMapComponent, StateDropdownStub],
            imports: [
                ReactiveFormsModule,
                HttpClientTestingModule,
                TranslateModule.forRoot(),
                UiLoaderStub
            ],
            providers: [
                { provide: WorkflowEventMapApi, useValue: workflowEventMapApi },
                { provide: StateStore, useValue: stateStore },
                { provide: ToastNotificationService, useValue: toast }
            ]
        });
        await TestBed.compileComponents();

        fixture = TestBed.createComponent(WorkflowEventStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();
    });

    it('should group all 8 events into a PR group and an agent group', () => {
        expect(component.eventGroups.length).toBe(2);
        expect(component.eventGroups[0].events).toEqual(['done']);
        expect(component.eventGroups[1].events.length).toBe(7);
        const total = component.eventGroups.reduce((sum, group) => sum + group.events.length, 0);
        expect(total).toBe(8);
    });

    it('renders both group headings and all 8 event rows, with done under the PR group', () => {
        const headingTexts = fixture.debugElement
            .queryAll(By.css('th[scope="colgroup"]'))
            .map(el => (el.nativeElement as HTMLElement).textContent?.trim());
        expect(headingTexts).toEqual([
            'WORKFLOW.STATE_MAP.GROUP_PR',
            'WORKFLOW.STATE_MAP.GROUP_AGENT'
        ]);

        const bodies = fixture.debugElement.queryAll(By.css('tbody'));
        expect(bodies.length).toBe(2);
        const firstGroupRows = bodies[0].queryAll(By.css('tr'));
        // First row in the first tbody is the heading row, the rest are event rows.
        expect(firstGroupRows.length - 1).toBe(1);

        const allRows = fixture.debugElement.queryAll(By.css('tbody tr'));
        expect(allRows.length).toBe(8 + 2);
    });

    it('shows a per-event description for each row', () => {
        const descriptions = fixture.debugElement
            .queryAll(By.css('td .text-sm'))
            .map(el => (el.nativeElement as HTMLElement).textContent?.trim());
        expect(descriptions).toContain('WORKFLOW.STATE_MAP.EVENT_DESC.DONE');
        expect(descriptions).toContain('WORKFLOW.STATE_MAP.EVENT_DESC.PR_OPEN');
    });

    it('does not render the raw slug as row text but keeps it as a title attribute', () => {
        const rowsText = fixture.debugElement
            .queryAll(By.css('tbody tr'))
            .map(el => (el.nativeElement as HTMLElement).textContent ?? '');
        expect(rowsText.some(text => text.includes('pr_open'))).toBe(false);

        const prOpenLabel = fixture.debugElement
            .queryAll(By.css('span[title]'))
            .find(el => el.nativeElement.getAttribute('title') === 'pr_open');
        expect(prOpenLabel).toBeTruthy();
    });

    it('should pre-populate dropdowns for mapped events', () => {
        workflowEventMapApi.load$.mockReturnValue(
            of([
                { idProject: 10, event: 'in_progress', idState: 2 },
                { idProject: 10, event: 'done', idState: 3 }
            ])
        );

        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        component.ngOnInit();
        fixture.detectChanges();

        const inProgressIndex = component.eventRowIndex('in_progress' as any);
        const doneIndex = component.eventRowIndex('done' as any);
        expect(component.getMappingControl(inProgressIndex).value).toBe(2);
        expect(component.getMappingControl(doneIndex).value).toBe(3);
    });

    it('should show project states in dropdown options', () => {
        expect(component.states().length).toBe(3);
        expect(component.states()[0].name).toBe('To Do');
    });

    // State CRUD lives in a sibling panel on the same settings page, so states$
    // re-emits while this form is on screen. Rebuilding the form on that emission
    // threw away a pick the user had not saved yet.
    it('keeps an unsaved pick when the project states change', () => {
        const states$ = new BehaviorSubject(mockStates);
        stateStore.statesByProject$.mockReturnValue(states$);

        fixture = TestBed.createComponent(WorkflowEventStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();

        const doneIndex = component.eventRowIndex('done' as any);
        component.getMappingControl(doneIndex).setValue(3, { emitEvent: false });

        states$.next([...mockStates, { ...mockStates[0], idState: 4, name: 'Blocked' }]);
        fixture.detectChanges();

        expect(component.getMappingControl(doneIndex).value).toBe(3);
        expect(component.states().length).toBe(4);
    });

    // Deleting a mapped state in the sibling panel repoints the mapping server-side
    // (migrate) or clears it (unassign); the form held the removed id and rendered an
    // empty row until a page reload.
    it('re-reads the mappings when a mapped state disappears from the states list', () => {
        const states$ = new BehaviorSubject(mockStates);
        stateStore.statesByProject$.mockReturnValue(states$);
        workflowEventMapApi.load$.mockReturnValue(
            of([{ idProject: 10, event: 'done', idState: 3 }])
        );

        fixture = TestBed.createComponent(WorkflowEventStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();

        const doneIndex = component.eventRowIndex('done' as any);
        expect(component.getMappingControl(doneIndex).value).toBe(3);

        // state 3 deleted with migrateTo=2 → server repointed the mapping
        workflowEventMapApi.load$.mockReturnValue(
            of([{ idProject: 10, event: 'done', idState: 2 }])
        );
        states$.next(mockStates.filter(state => state.idState !== 3));
        fixture.detectChanges();

        expect(component.getMappingControl(doneIndex).value).toBe(2);
    });

    it('does not re-read the mappings when the states list stays valid', () => {
        const states$ = new BehaviorSubject(mockStates);
        stateStore.statesByProject$.mockReturnValue(states$);

        fixture = TestBed.createComponent(WorkflowEventStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();
        const loadCalls = workflowEventMapApi.load$.mock.calls.length;

        states$.next([...mockStates, { ...mockStates[0], idState: 4, name: 'Blocked' }]);
        fixture.detectChanges();

        expect(workflowEventMapApi.load$.mock.calls.length).toBe(loadCalls);
    });

    // Rows auto-save on change (pick = commit); each change sends the full
    // non-null mapping set. No bulk save button / success toast anymore — a
    // per-row status chip reflects Saving/Saved/Error.
    it('auto-saves only non-null mappings when a row in the PR group changes', () => {
        const doneIndex = component.eventRowIndex('done' as any);

        component.getMappingControl(doneIndex).setValue(3);

        expect(workflowEventMapApi.replace$).toHaveBeenLastCalledWith(10, [
            { event: 'done', idState: 3 }
        ]);
    });

    it('auto-saves the full flat mapping list when a row in the agent group changes', () => {
        const inProgressIndex = component.eventRowIndex('in_progress' as any);
        const doneIndex = component.eventRowIndex('done' as any);

        component.getMappingControl(doneIndex).setValue(3);
        component.getMappingControl(inProgressIndex).setValue(2);

        expect(workflowEventMapApi.replace$).toHaveBeenLastCalledWith(10, [
            { event: 'done', idState: 3 },
            { event: 'in_progress', idState: 2 }
        ]);
    });

    it('sends an empty array when the last mapping is cleared', () => {
        const queuedIndex = component.eventRowIndex('queued' as any);
        component.getMappingControl(queuedIndex).setValue(2); // save [{queued:2}]
        component.getMappingControl(queuedIndex).setValue(null); // save []
        expect(workflowEventMapApi.replace$).toHaveBeenLastCalledWith(10, []);
    });

    it('marks the row Saved on successful auto-save (no success toast)', () => {
        const doneIndex = component.eventRowIndex('done' as any);
        component.getMappingControl(doneIndex).setValue(3);
        expect(component.rowSaveStatus(doneIndex)).toBe(UiSaveState.Saved);
        expect(toast.showSuccess).not.toHaveBeenCalled();
    });

    it('shows an error toast and marks the row Error on auto-save failure', () => {
        workflowEventMapApi.replace$.mockReturnValue(throwError(() => new Error('fail')));
        const doneIndex = component.eventRowIndex('done' as any);
        component.getMappingControl(doneIndex).setValue(3);
        expect(toast.showError).toHaveBeenCalledWith('WORKFLOW.STATE_MAP.SAVE_ERROR');
        expect(component.rowSaveStatus(doneIndex)).toBe(UiSaveState.Error);
    });
});
