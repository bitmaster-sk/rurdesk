import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ProjectSeverityComponent } from './project-severity.component';
import { SeverityApi } from '../../api/severity.api.service';
import { SeverityStore } from '../../store/severity.store';
import { ProjectService } from '../../../project/project.service';
import { WindowService } from '../../../shared/window/window.service';

/**
 * Delete flow (R-safe-state-severity-delete, task 6): click trash → fetch
 * usage → open dialog → confirm sends the migration choice → refresh store.
 */
describe('ProjectSeverityComponent delete flow (browser)', () => {
    const severity = (id: number, rank: number) => ({
        idSeverity: id,
        idProject: 10,
        title: `S${id}`,
        color: '#fff',
        protected: false,
        orderRank: rank
    });

    let severityApi: { load$: any; update$: any; delete$: any; usage$: any };
    let severityStore: { load: any };

    function setup(usage = { issues: 0, isProjectDefault: false }) {
        severityApi = {
            load$: vi.fn().mockReturnValue(of([severity(1, 1), severity(2, 2), severity(3, 3)])),
            update$: vi.fn().mockReturnValue(of(null)),
            delete$: vi.fn().mockReturnValue(of(undefined)),
            usage$: vi.fn().mockReturnValue(of(usage))
        };
        severityStore = { load: vi.fn() };

        TestBed.configureTestingModule({
            declarations: [ProjectSeverityComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [
                { provide: SeverityApi, useValue: severityApi },
                { provide: SeverityStore, useValue: severityStore },
                {
                    provide: ProjectService,
                    useValue: { updateProject: vi.fn().mockReturnValue(of({})) }
                }
            ]
        });
        TestBed.overrideComponent(ProjectSeverityComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectSeverityComponent);
        fixture.componentRef.setInput('project', {
            idProject: 10,
            name: 'P',
            idSeverityDefault: 2
        });
        fixture.detectChanges();
        return fixture;
    }

    it('fetches usage then opens the dialog', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteSeverity(severity(1, 1));

        expect(severityApi.usage$).toHaveBeenCalledWith(10, 1);
        expect(component.isDeleteDialogVisible()).toBe(true);
        expect(component.hasDeleteUsage()).toBe(true);
    });

    it('sends the migration choice and closes the dialog on success', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteSeverity(severity(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(severityApi.delete$).toHaveBeenCalledWith(10, 1, { migrateTo: 2 });
        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(false);
        expect(component.severities().map((s: any) => s.idSeverity)).toEqual([2, 3]);
        expect(severityStore.load).toHaveBeenCalled();
    });

    it('keeps the dialog open and stops loading on error', () => {
        const fixture = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;
        severityApi.delete$ = vi.fn().mockReturnValue(throwError(() => new Error('boom')));

        component.onDeleteSeverity(severity(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(true);
    });

    it('sends a bare delete (no intent) when there is zero usage', () => {
        const fixture = setup({ issues: 0, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteSeverity(severity(1, 1));
        component.onConfirmDelete({ migrateTo: null });

        expect(severityApi.delete$).toHaveBeenCalledWith(10, 1, undefined);
    });

    it('refreshes the local default when the deleted severity was the project default', () => {
        const fixture = setup({ issues: 0, isProjectDefault: true });
        const component = fixture.componentInstance as any;
        const originalDefault = component.project().idSeverityDefault;

        component.onDeleteSeverity(severity(2, 2));
        component.onConfirmDelete({ migrateTo: 3 });

        expect(component.form.value.idSeverityDefault).toBe(3);
        expect(component.project().idSeverityDefault).toBe(originalDefault);
    });

    it('does not mutate the project input when the default changes', () => {
        const fixture = setup();
        const component = fixture.componentInstance as any;
        const original = { ...component.project() };

        component.form.patchValue({ idSeverityDefault: 3 });
        component.onProjectSave();

        expect(component.project()).toEqual(original);
    });
});
