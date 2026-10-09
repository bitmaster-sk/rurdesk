import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ProjectIssueTypeComponent } from './project-issue-type.component';
import { IssueTypeApi } from '../../api/issue-type.api.service';
import { IssueTypeStore } from '../../store/issue-type.store';
import { Project } from '../../../project/model/project.model';
import { WindowService } from '../../../shared/window/window.service';

describe('ProjectIssueTypeComponent (browser)', () => {
    const issueType = (id: number, rank: number) => ({
        idIssueType: id,
        idProject: 10,
        name: `T${id}`,
        protected: false,
        orderRank: rank
    });

    let issueTypeApi: {
        load$: any;
        update$: any;
        delete$: any;
        usage$: any;
    };
    let issueTypeStore: { load: any };

    function setup(usage = { issues: 0, isProjectDefault: false }) {
        issueTypeApi = {
            load$: vi.fn().mockReturnValue(of([issueType(1, 1), issueType(2, 2), issueType(3, 3)])),
            update$: vi.fn().mockReturnValue(of(null)),
            delete$: vi.fn().mockReturnValue(of(undefined)),
            usage$: vi.fn().mockReturnValue(of(usage))
        };
        issueTypeStore = { load: vi.fn() };

        TestBed.configureTestingModule({
            declarations: [ProjectIssueTypeComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [
                { provide: IssueTypeApi, useValue: issueTypeApi },
                { provide: IssueTypeStore, useValue: issueTypeStore }
            ]
        });
        TestBed.overrideComponent(ProjectIssueTypeComponent, {
            set: { template: '', providers: [{ provide: WindowService, useValue: {} }] }
        });
        const fixture = TestBed.createComponent(ProjectIssueTypeComponent);
        const input: Project = {
            idProject: 10,
            name: 'P',
            color: '#000',
            idIssueTypeDefault: 2
        };
        fixture.componentRef.setInput('project', input);
        fixture.detectChanges();
        return { fixture, input };
    }

    it('shows only the types of the bound project', () => {
        const { fixture } = setup();
        const component = fixture.componentInstance as any;
        expect(component.issueTypes().map((t: any) => t.idIssueType)).toEqual([1, 2, 3]);
    });

    it('offers every other type as a migration target, mapped to id/label', () => {
        const { fixture } = setup({ issues: 2, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteIssueType(issueType(2, 2));

        expect(component.deleteOptions()).toEqual([
            { id: 1, label: 'T1' },
            { id: 3, label: 'T3' }
        ]);
    });

    it('fetches usage then opens the dialog', () => {
        const { fixture } = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteIssueType(issueType(1, 1));

        expect(issueTypeApi.usage$).toHaveBeenCalledWith(10, 1);
        expect(component.isDeleteDialogVisible()).toBe(true);
        expect(component.hasDeleteUsage()).toBe(true);
    });

    it('sends the migration choice and closes the dialog on success', () => {
        const { fixture } = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteIssueType(issueType(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(issueTypeApi.delete$).toHaveBeenCalledWith(10, 1, { migrateTo: 2 });
        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(false);
        expect(component.issueTypes().map((t: any) => t.idIssueType)).toEqual([2, 3]);
        expect(issueTypeStore.load).toHaveBeenCalled();
    });

    it('sends a bare delete (no intent) when there is zero usage', () => {
        const { fixture } = setup({ issues: 0, isProjectDefault: false });
        const component = fixture.componentInstance as any;

        component.onDeleteIssueType(issueType(1, 1));
        component.onConfirmDelete({ migrateTo: null });

        expect(issueTypeApi.delete$).toHaveBeenCalledWith(10, 1, undefined);
    });

    it('keeps the dialog open and stops loading on error', () => {
        const { fixture } = setup({ issues: 3, isProjectDefault: false });
        const component = fixture.componentInstance as any;
        issueTypeApi.delete$ = vi.fn().mockReturnValue(throwError(() => new Error('boom')));

        component.onDeleteIssueType(issueType(1, 1));
        component.onConfirmDelete({ migrateTo: 2 });

        expect(component.isDeleting()).toBe(false);
        expect(component.isDeleteDialogVisible()).toBe(true);
    });

    it('emits the migrated default without touching the input or calling an update', () => {
        const { fixture, input } = setup({ issues: 0, isProjectDefault: true });
        const component = fixture.componentInstance as any;
        const migrated: Project[] = [];
        component.defaultMigrated.subscribe((project: Project) => migrated.push(project));

        component.onDeleteIssueType(issueType(2, 2));
        component.onConfirmDelete({ migrateTo: 3 });

        expect(migrated).toEqual([
            { idProject: 10, name: 'P', color: '#000', idIssueTypeDefault: 3 }
        ]);
        expect(input.idIssueTypeDefault).toBe(2); // the input object is untouched
        expect(issueTypeApi.update$).not.toHaveBeenCalledWith(
            expect.objectContaining({ idIssueTypeDefault: 3 })
        ); // no project PATCH from the delete flow (update$ remains the type-rank PUT)
        expect(component.idIssueTypeDefaultControl.value).toBe(3); // patchValue contract
    });

    it('does not emit a migration when the deleted type was not the default', () => {
        const { fixture } = setup({ issues: 0, isProjectDefault: false });
        const component = fixture.componentInstance as any;
        const migrated: Project[] = [];
        component.defaultMigrated.subscribe((project: Project) => migrated.push(project));

        component.onDeleteIssueType(issueType(1, 1)); // default is type 2
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
        component.form.controls.idIssueTypeDefault.patchValue(3, { emitEvent: false });
        component.onProjectSave();

        expect(saved).toEqual([{ idProject: 10, name: 'P', color: '#000', idIssueTypeDefault: 3 }]);
        expect(saved[0]).not.toBe(input); // a copy, not the shared object
        expect(input.idIssueTypeDefault).toBe(2); // the input object is unchanged
    });

    it('persists the new rank of the moved row on reorder', () => {
        const { fixture } = setup();
        const component = fixture.componentInstance as any;

        component.onReorder({ previousIndex: 2, currentIndex: 0 });

        expect(component.issueTypes().map((t: any) => t.idIssueType)).toEqual([3, 1, 2]);
        expect(issueTypeApi.update$).toHaveBeenCalledWith(
            expect.objectContaining({ idIssueType: 3, orderRank: 1 })
        );
    });
});
