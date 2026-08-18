import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { BehaviorSubject, of, throwError } from 'rxjs';
import { TranslateModule } from '@ngx-translate/core';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { AgentPhaseStateMapComponent } from './agent-phase-state-map.component';
import { PhaseStateMapApi } from '../../api/phase-state-map.api.service';
import { StateStore } from '../../../state/store/state.store';
import { AGENT_PHASES } from '../../model/phase-state-mapping.model';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';

describe('AgentPhaseStateMapComponent', () => {
    let component: AgentPhaseStateMapComponent;
    let fixture: ComponentFixture<AgentPhaseStateMapComponent>;
    let phaseStateMapApi: any;
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
        phaseStateMapApi = { load$: vi.fn(), replace$: vi.fn() };
        stateStore = { statesByProject$: vi.fn() };
        toast = { showSuccess: vi.fn(), showError: vi.fn() };

        phaseStateMapApi.load$.mockReturnValue(of([]));
        stateStore.statesByProject$.mockReturnValue(of(mockStates));
        phaseStateMapApi.replace$.mockReturnValue(of([]));

        TestBed.configureTestingModule({
            declarations: [AgentPhaseStateMapComponent],
            imports: [ReactiveFormsModule, HttpClientTestingModule, TranslateModule.forRoot()],
            providers: [
                { provide: PhaseStateMapApi, useValue: phaseStateMapApi },
                { provide: StateStore, useValue: stateStore },
                { provide: ToastNotificationService, useValue: toast }
            ]
        });
        // Logic-focused spec: the template projects ui-select (a CVA) into a
        // native <table> now, which would need UiModule + CDK overlay to render.
        // These tests assert component state/mappings only, so blank the template.
        TestBed.overrideComponent(AgentPhaseStateMapComponent, { set: { template: '' } });
        await TestBed.compileComponents();

        fixture = TestBed.createComponent(AgentPhaseStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();
    });

    it('should load and display all 8 phases', () => {
        expect(component.phases.length).toBe(AGENT_PHASES.length);
        expect(component.phases.length).toBe(8);
    });

    it('should pre-populate dropdowns for mapped phases', () => {
        phaseStateMapApi.load$.mockReturnValue(
            of([
                { idProject: 10, phase: 'in_progress', idState: 2 },
                { idProject: 10, phase: 'done', idState: 3 }
            ])
        );

        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        component.ngOnInit();
        fixture.detectChanges();

        const inProgressIndex = AGENT_PHASES.indexOf('in_progress' as any);
        const doneIndex = AGENT_PHASES.indexOf('done' as any);
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

        fixture = TestBed.createComponent(AgentPhaseStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();

        const doneIndex = AGENT_PHASES.indexOf('done' as any);
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
        phaseStateMapApi.load$.mockReturnValue(of([{ idProject: 10, phase: 'done', idState: 3 }]));

        fixture = TestBed.createComponent(AgentPhaseStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();

        const doneIndex = AGENT_PHASES.indexOf('done' as any);
        expect(component.getMappingControl(doneIndex).value).toBe(3);

        // state 3 deleted with migrateTo=2 → server repointed the mapping
        phaseStateMapApi.load$.mockReturnValue(of([{ idProject: 10, phase: 'done', idState: 2 }]));
        states$.next(mockStates.filter(state => state.idState !== 3));
        fixture.detectChanges();

        expect(component.getMappingControl(doneIndex).value).toBe(2);
    });

    it('does not re-read the mappings when the states list stays valid', () => {
        const states$ = new BehaviorSubject(mockStates);
        stateStore.statesByProject$.mockReturnValue(states$);

        fixture = TestBed.createComponent(AgentPhaseStateMapComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('project', { idProject: 10, name: 'Test', color: '#000' });
        fixture.detectChanges();
        const loadCalls = phaseStateMapApi.load$.mock.calls.length;

        states$.next([...mockStates, { ...mockStates[0], idState: 4, name: 'Blocked' }]);
        fixture.detectChanges();

        expect(phaseStateMapApi.load$.mock.calls.length).toBe(loadCalls);
    });

    // Rows auto-save on change (pick = commit); each change sends the full
    // non-null mapping set. No bulk save button / success toast anymore — a
    // per-row status chip reflects Saving/Saved/Error.
    it('auto-saves only non-null mappings when a row changes', () => {
        const inProgressIndex = AGENT_PHASES.indexOf('in_progress' as any);
        const doneIndex = AGENT_PHASES.indexOf('done' as any);

        component.getMappingControl(inProgressIndex).setValue(2);
        component.getMappingControl(doneIndex).setValue(3);

        // Last auto-save carries both non-null rows; null rows are filtered out.
        expect(phaseStateMapApi.replace$).toHaveBeenLastCalledWith(10, [
            { phase: 'in_progress', idState: 2 },
            { phase: 'done', idState: 3 }
        ]);
    });

    it('sends an empty array when the last mapping is cleared', () => {
        const queuedIndex = AGENT_PHASES.indexOf('queued' as any);
        component.getMappingControl(queuedIndex).setValue(2); // save [{queued:2}]
        component.getMappingControl(queuedIndex).setValue(null); // save []
        expect(phaseStateMapApi.replace$).toHaveBeenLastCalledWith(10, []);
    });

    it('marks the row Saved on successful auto-save (no success toast)', () => {
        const doneIndex = AGENT_PHASES.indexOf('done' as any);
        component.getMappingControl(doneIndex).setValue(3);
        expect(component.rowSaveStatus(doneIndex)).toBe(UiSaveState.Saved);
        expect(toast.showSuccess).not.toHaveBeenCalled();
    });

    it('shows an error toast and marks the row Error on auto-save failure', () => {
        phaseStateMapApi.replace$.mockReturnValue(throwError(() => new Error('fail')));
        const doneIndex = AGENT_PHASES.indexOf('done' as any);
        component.getMappingControl(doneIndex).setValue(3);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.PHASE_STATE_MAP.SAVE_ERROR');
        expect(component.rowSaveStatus(doneIndex)).toBe(UiSaveState.Error);
    });
});
