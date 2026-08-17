import { ChangeDetectionStrategy, Component } from '@angular/core';
import { Router } from '@angular/router';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { Project } from '../../model/project.model';
import { ProjectService } from '../../project.service';

@Component({
    selector: 'app-project-form-window',
    templateUrl: './project-form-window.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ProjectFormWindowComponent {
    private readonly winRef = inject(WindowReference);
    public readonly winCfg = inject<WindowConfig<ProjectWindowData>>(WindowConfig);
    private readonly sProject = inject(ProjectService);
    private readonly router = inject(Router);

    public onSave(project: Project): void {
        this.saveProject(project).subscribe(savedProject => {
            this.winRef.close(savedProject);
        });
    }

    public onSaveGenerate(project: Project): void {
        this.saveProject(project).subscribe(savedProject => {
            this.winRef.close(savedProject);
            this.router.navigate(['/project', savedProject.idProject, 'project-builder']);
        });
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    private saveProject(project: Project): Observable<Project> {
        return project.idProject
            ? this.sProject.updateProject(project)
            : this.sProject.insertProject(project);
    }
}
