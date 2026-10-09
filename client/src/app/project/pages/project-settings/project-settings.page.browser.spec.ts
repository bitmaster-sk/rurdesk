import { TestBed } from '@angular/core/testing';
import { BehaviorSubject, of, throwError } from 'rxjs';
import { Project } from '../../model/project.model';
import { ProjectStore } from '../../project.store';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { ProjectSettingsPage } from './project-settings.page';

/**
 * Wiring contract: the three default-panels PATCH through ProjectStore.update(),
 * and the delete-migration emission only refreshes the store locally (the
 * backend already repointed the default inside the delete-transaction).
 */
describe('ProjectSettingsPage (browser)', () => {
    const project: Project = {
        idProject: 10,
        name: 'P',
        color: '#000',
        idStateDefault: 2,
        idSeverityDefault: 3,
        idIssueTypeDefault: 1
    };

    let project$: BehaviorSubject<Project | null>;
    let update: ReturnType<typeof vi.fn>;
    let setProject: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        project$ = new BehaviorSubject<Project | null>(project);
        update = vi.fn().mockReturnValue(of({ ...project }));
        setProject = vi.fn();
        showError = vi.fn();

        await TestBed.configureTestingModule({
            declarations: [ProjectSettingsPage],
            providers: [
                { provide: ProjectStore, useValue: { project$, update, setProject } },
                { provide: ToastNotificationService, useValue: { showError } }
            ]
        })
            .overrideComponent(ProjectSettingsPage, { set: { template: '' } })
            .compileComponents();
    });

    function render(): { component: ProjectSettingsPage; detectChanges: () => void } {
        const fixture = TestBed.createComponent(ProjectSettingsPage);
        fixture.detectChanges();
        return {
            component: fixture.componentInstance,
            detectChanges: () => fixture.detectChanges()
        };
    }

    it('routes a panel save through ProjectStore.update()', () => {
        const { component } = render();
        const saved: Project = { ...project, idStateDefault: 5 };

        (component as any).onStateSave(saved);

        expect(update).toHaveBeenCalledWith(saved);
        expect(setProject).not.toHaveBeenCalled();
    });

    it('routes a migration through ProjectStore.setProject() and never PATCHes', () => {
        const { component } = render();
        const migrated: Project = { ...project, idStateDefault: 4 };

        (component as any).onDefaultMigrated(migrated);

        expect(setProject).toHaveBeenCalledWith(migrated);
        expect(update).not.toHaveBeenCalled();
    });

    it('pins the panel status to Error and shows the toast when a save fails', () => {
        const { component } = render();
        update.mockReturnValue(throwError(() => new Error('boom')));

        (component as any).onSeveritySave({ ...project, idSeverityDefault: 1 });

        expect((component as any).severitySaveStatus()).toBe(UiSaveState.Error);
        expect(showError).toHaveBeenCalledWith('PROJECT.SAVE_ERROR');
    });
});
