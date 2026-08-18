import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { Router } from '@angular/router';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { ProjectFormWindowComponent } from './project-form-window.component';
import { ProjectService } from '../../project.service';
import { Project, ProjectInsert } from '../../model/project.model';

describe('ProjectFormWindowComponent', () => {
    let close: ReturnType<typeof vi.fn>;
    let navigate: ReturnType<typeof vi.fn>;
    let insertProject: ReturnType<typeof vi.fn>;
    let updateProject: ReturnType<typeof vi.fn>;
    let component: ProjectFormWindowComponent;

    // idProject is absent on purpose — the component treats its absence as "insert, not update".
    const draftProject: ProjectInsert = { name: 'New', color: '#123456' };
    const newProject = draftProject as Project;
    const savedProject: Project = { idProject: 7, name: 'New', color: '#123456' };
    const existingProject: Project = { idProject: 3, name: 'Edit', color: '#654321' };

    beforeEach(() => {
        close = vi.fn();
        navigate = vi.fn();
        insertProject = vi.fn().mockReturnValue(of(savedProject));
        updateProject = vi.fn().mockReturnValue(of(existingProject));

        const injector = Injector.create({
            providers: [
                { provide: WindowReference, useValue: { close } },
                { provide: WindowConfig, useValue: {} },
                { provide: ProjectService, useValue: { insertProject, updateProject } },
                { provide: Router, useValue: { navigate } }
            ]
        });
        component = runInInjectionContext(injector, () => new ProjectFormWindowComponent());
    });

    it('onSave inserts a new project and closes the window without navigation', () => {
        component.onSave(newProject);

        expect(insertProject).toHaveBeenCalledWith(newProject);
        expect(close).toHaveBeenCalledWith(savedProject);
        expect(navigate).not.toHaveBeenCalled();
    });

    it('onSave updates an existing project and closes the window without navigation', () => {
        component.onSave(existingProject);

        expect(updateProject).toHaveBeenCalledWith(existingProject);
        expect(close).toHaveBeenCalledWith(existingProject);
        expect(navigate).not.toHaveBeenCalled();
    });

    it('onSaveGenerate inserts the project, closes the window and navigates to the project builder', () => {
        component.onSaveGenerate(newProject);

        expect(insertProject).toHaveBeenCalledWith(newProject);
        expect(close).toHaveBeenCalledWith(savedProject);
        expect(navigate).toHaveBeenCalledWith(['/project', 7, 'project-builder']);
    });

    it('onCancel closes the window with null', () => {
        component.onCancel();

        expect(close).toHaveBeenCalledWith(null);
        expect(navigate).not.toHaveBeenCalled();
    });
});
