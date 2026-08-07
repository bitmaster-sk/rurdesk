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
    constructor(
        private winRef: WindowReference,
        public winCfg: WindowConfig,
        private sProject: ProjectService,
        private router: Router
    ) {}

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

    private saveProject(project: Project) {
        return project.idProject
            ? this.sProject.updateProject(project)
            : this.sProject.insertProject(project);
    }
}
