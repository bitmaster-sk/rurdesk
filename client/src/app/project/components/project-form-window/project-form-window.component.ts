import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { Observable } from 'rxjs';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { Project } from '../../model/project.model';
import { ProjectApi } from '../../api/project.api.service';

export interface ProjectWindowData {
    project?: Project;
}

@Component({
    selector: 'app-project-form-window',
    templateUrl: './project-form-window.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectFormWindowComponent {
    private readonly winRef = inject(WindowReference);
    public readonly winCfg = inject<WindowConfig<ProjectWindowData>>(WindowConfig);
    private readonly projectApi = inject(ProjectApi);
    private readonly router = inject(Router);

    public onSave(project: Project): void {
        this.saveProject(project).subscribe(savedProject => {
            this.winRef.close(savedProject);
        });
    }

    public onSaveGenerate(project: Project): void {
        this.saveProject(project).subscribe(savedProject => {
            this.winRef.close(savedProject);
            void this.router.navigate(['/project', savedProject.idProject, 'project-builder']);
        });
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    private saveProject(project: Project): Observable<Project> {
        return project.idProject
            ? this.projectApi.update$(project)
            : this.projectApi.insert$(project);
    }

    /** Partial: opening the window for a new project supplies a bare draft. */
    public get project(): Partial<Project> {
        return this.winCfg.data?.project ?? {};
    }
}
