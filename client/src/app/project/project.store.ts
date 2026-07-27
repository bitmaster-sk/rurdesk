import { inject, Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, tap } from 'rxjs/operators';
import { Project } from './model/project.model';
import { ProjectService } from './project.service';

@Injectable({
    providedIn: 'root'
})
export class ProjectStore {
    private project = new BehaviorSubject<Project>(null);

    public project$ = this.project.asObservable().pipe(filter(project => !!project));

    private readonly sProject = inject(ProjectService);

    public load(idProject: number): void {
        // A failure here leaves project$ never emitting; the global error toast
        // tells the user why the page behind the resolver stayed empty.
        this.sProject.loadProject(idProject).subscribe(project => this.project.next(project));
    }

    public update(project: Project): Observable<Project> {
        return this.sProject
            .updateProject(project)
            .pipe(tap(savedProject => this.project.next(savedProject)));
    }
}
