import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ProjectStateComponent } from './project-state.component';
import { StateApi } from '../../api/state.api.service';
import { StateStore } from '../../store/state.store';
import { Project } from '../../../project/model/project.model';
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
                { provide: StateStore, useValue: { load: vi.fn() } }
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
                { provide: StateStore, useValue: stateStore }
            ]
        });
        TestBed.overrideComponent(ProjectStateComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectStateComponent);
        const input: Project = {
            idProject: 10,
            name: 'P',
            color: '#000',
            idStateDefault: 2
        };
        fixture.componentRef.setInput('project', input);
        fixture.detectChanges();
        return { fixture, input };
    }

    it('fetches usage then opens the dialog', () => {
        const { fixture } = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;

        component.onDeleteState(state(1, 1));

        expect(stateApi.usage$).toHaveBeenCalledWith(10, 1);
        expect(component.isDeleteDialogVisible()).toBe(true);
        expect(component.hasDeleteUsage()).toBe(true);
    });

    it('sends the migration choice and closes the dialog on success', () => {
        const { fixture } = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
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
        const { fixture } = setup({ issues: 3, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;
        stateApi.delete$ = vi.fn().mockReturnValue(throwError(() => new Error('boom')));

        component.onDeleteState(state(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(true);
    });

    it('sends a bare delete (no intent) when there is zero usage', () => {
        const { fixture } = setup({ issues: 0, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;

        component.onDeleteState(state(1, 1));
        component.onConfirmDelete({ migrateTo: null });

        expect(stateApi.delete$).toHaveBeenCalledWith(10, 1, undefined);
    });

    it('emits the migrated default without touching the input or calling an update', () => {
        const { fixture, input } = setup({ issues: 0, isProjectDefault: true, agentPhases: 0 });
        const component = fixture.componentInstance as any;
        const migrated: Project[] = [];
        component.defaultMigrated.subscribe((project: Project) => migrated.push(project));

        component.onDeleteState(state(2, 2));
        component.onConfirmDelete({ migrateTo: 3 });

        expect(migrated).toEqual([{ idProject: 10, name: 'P', color: '#000', idStateDefault: 3 }]);
        expect(input.idStateDefault).toBe(2); // the input object is untouched
        expect(stateApi.update$).not.toHaveBeenCalled(); // no second PATCH
        expect(component.form.value.idStateDefault).toBe(3); // patchValue contract
    });

    it('does not emit a migration when the deleted state was not the default', () => {
        const { fixture } = setup({ issues: 0, isProjectDefault: false, agentPhases: 0 });
        const component = fixture.componentInstance as any;
        const migrated: Project[] = [];
        component.defaultMigrated.subscribe((project: Project) => migrated.push(project));

        component.onDeleteState(state(1, 1)); // default is state 2
        component.onConfirmDelete({ migrateTo: 3 });

        expect(migrated).toEqual([]);
    });

    it('save emits a copy of the project and leaves the input unchanged', () => {
        const { fixture, input } = setup();
        const component = fixture.componentInstance as any;
        const saved: Project[] = [];
        component.save.subscribe((project: Project) => saved.push(project));

        // patchValue (emitEvent:false) keeps the auto-save valueChanges from
        // firing — the manual call below is the single emission under test.
        component.form.controls.idStateDefault.patchValue(3, { emitEvent: false });
        component.onProjectSave();

        expect(saved).toEqual([{ idProject: 10, name: 'P', color: '#000', idStateDefault: 3 }]);
        expect(saved[0]).not.toBe(input); // a copy, not the shared object
        expect(input.idStateDefault).toBe(2); // the input object is unchanged
        expect(stateApi.update$).not.toHaveBeenCalled(); // the child never PATCHes
    });
});
