import { of } from 'rxjs';
import type { Router } from '@angular/router';
import type { WindowConfig } from 'src/app/shared/window/entity/window-config';
import type { WindowReference } from 'src/app/shared/window/window.reference';
import { ProjectFormWindowComponent } from './project-form-window.component';
import type { ProjectService } from '../../project.service';
import { Project } from '../../model/project.model';

describe('ProjectFormWindowComponent', () => {
    let close: ReturnType<typeof vi.fn>;
    let navigate: ReturnType<typeof vi.fn>;
    let insertProject: ReturnType<typeof vi.fn>;
    let updateProject: ReturnType<typeof vi.fn>;
    let component: ProjectFormWindowComponent;

    const newProject = { name: 'New' } as Project;
    const savedProject = { idProject: 7, name: 'New' } as Project;
    const existingProject = { idProject: 3, name: 'Edit' } as Project;

    beforeEach(() => {
        close = vi.fn();
        navigate = vi.fn();
        insertProject = vi.fn().mockReturnValue(of(savedProject));
        updateProject = vi.fn().mockReturnValue(of(existingProject));

        component = new ProjectFormWindowComponent(
            { close } as unknown as WindowReference,
            {} as WindowConfig,
            { insertProject, updateProject } as unknown as ProjectService,
            { navigate } as unknown as Router
        );
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
