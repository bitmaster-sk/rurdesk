import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ProjectStateComponent } from './project-state.component';
import { StateApi } from '../../api/state.api.service';
import { StateStore } from '../../store/state.store';
import { ProjectService } from '../../../project/project.service';
import { WindowService } from '../../../shared/window/window.service';

/**
 * Behavioural guard for the p-table → CDK row-reorder migration (R9): CDK's
 * cdkDropListDropped does NOT mutate the array (unlike PrimeNG onRowReorder), so
 * the handler must reorder a copy + set the signal before reading currentIndex.
 * A regression here silently persists the wrong orderRank.
 */
describe('ProjectStateComponent reorder (browser)', () => {
    const state = (id: number, rank: number) => ({
        idState: id,
        idProject: 10,
        name: `S${id}`,
        start: false,
        final: false,
        protected: false,
        orderRank: rank
    });

    let stateApi: { load$: any; update$: any; delete$: any; usage$: any };

    function setup() {
        stateApi = {
            load$: vi.fn().mockReturnValue(of([state(1, 1), state(2, 2), state(3, 3)])),
            update$: vi.fn().mockReturnValue(of(null)),
            delete$: vi.fn().mockReturnValue(of(null)),
            usage$: vi
                .fn()
                .mockReturnValue(of({ issues: 0, isProjectDefault: false, agentPhases: 0 }))
        };

        TestBed.configureTestingModule({
            declarations: [ProjectStateComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [
                { provide: StateApi, useValue: stateApi },
                { provide: StateStore, useValue: { load: vi.fn() } },
                {
                    provide: ProjectService,
                    useValue: { updateProject: vi.fn().mockReturnValue(of({})) }
                }
            ]
        });
        TestBed.overrideComponent(ProjectStateComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectStateComponent);
        fixture.componentRef.setInput('project', {
            idProject: 10,
            name: 'P',
            idStateDefault: null
        });
        fixture.detectChanges();
        return fixture;
    }

    it('moves the row to its new index and persists the moved row with the new rank', () => {
        const fixture = setup();
        const component = fixture.componentInstance as any;

        // drag row 0 (S1) down to index 2
        component.onReorder({ previousIndex: 0, currentIndex: 2 });

        const order = component.states().map((s: any) => s.idState);
        expect(order).toEqual([2, 3, 1]);

        // the moved row (S1) is now at index 2 → orderRank 3, and it (not some
        // stale item) is what gets persisted.
        const persisted = stateApi.update$.mock.calls[0][0];
        expect(persisted.idState).toBe(1);
        expect(persisted.orderRank).toBe(3);
    });
});

describe('ProjectStateComponent delete flow (browser)', () => {
    const state = (id: number, rank: number) => ({
        idState: id,
        idProject: 10,
        name: `S${id}`,
        start: false,
        final: false,
        protected: false,
        orderRank: rank
    });

    let stateApi: { load$: any; update$: any; delete$: any; usage$: any };
    let stateStore: { load: any };

    function setup(usage = { issues: 0, isProjectDefault: false, agentPhases: 0 }) {
        stateApi = {
            load$: vi.fn().mockReturnValue(of([state(1, 1), state(2, 2), state(3, 3)])),
            update$: vi.fn().mockReturnValue(of(null)),
            delete$: vi.fn().mockReturnValue(of(undefined)),
            usage$: vi.fn().mockReturnValue(of(usage))
        };
        stateStore = { load: vi.fn() };

        TestBed.configureTestingModule({
            declarations: [ProjectStateComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [
                { provide: StateApi, useValue: stateApi },
                { provide: StateStore, useValue: stateStore },
                {
                    provide: ProjectService,
                    useValue: { updateProject: vi.fn().mockReturnValue(of({})) }
                }
            ]
        });
        TestBed.overrideComponent(ProjectStateComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectStateComponent);
        fixture.componentRef.setInput('project', {
            idProject: 10,
            name: 'P',
            idStateDefault: 2
        });
        fixture.detectChanges();
        return fixture;
    }

    it('fetches usage then opens the dialog', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;

        component.onDeleteState(state(1, 1));

        expect(stateApi.usage$).toHaveBeenCalledWith(10, 1);
        expect(component.isDeleteDialogVisible()).toBe(true);
        expect(component.hasDeleteUsage()).toBe(true);
    });

    it('sends the migration choice and closes the dialog on success', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;

        component.onDeleteState(state(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(stateApi.delete$).toHaveBeenCalledWith(10, 1, { migrateTo: 2 });
        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(false);
        expect(component.states().map((s: any) => s.idState)).toEqual([2, 3]);
        expect(stateStore.load).toHaveBeenCalled();
    });

    it('keeps the dialog open and stops loading on error', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;
        stateApi.delete$ = vi.fn().mockReturnValue(throwError(() => new Error('boom')));

        component.onDeleteState(state(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(true);
    });

    it('sends a bare delete (no intent) when there is zero usage', () => {
        const fixture = setup({ issues: 0, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;

        component.onDeleteState(state(1, 1));
        component.onConfirmDelete({ migrateTo: null });

        expect(stateApi.delete$).toHaveBeenCalledWith(10, 1, undefined);
    });

    it('refreshes the local default when the deleted state was the project default', () => {
        const fixture = setup({ issues: 0, isProjectDefault: true, agentPhases: 0 });
        const component = fixture.componentInstance as any;
        const originalDefault = component.project().idStateDefault;

        component.onDeleteState(state(2, 2));
        component.onConfirmDelete({ migrateTo: 3 });

        expect(component.form.value.idStateDefault).toBe(3);
        expect(component.project().idStateDefault).toBe(originalDefault);
    });

    it('does not mutate the project input when the default changes', () => {
        const fixture = setup();
        const component = fixture.componentInstance as any;
        const original = { ...component.project() };

        component.form.patchValue({ idStateDefault: 3 });
        component.onProjectSave();

        expect(component.project()).toEqual(original);
    });
});
