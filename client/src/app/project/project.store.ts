import { inject, Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, tap } from 'rxjs/operators';
import { Project } from './model/project.model';
import { ProjectApi } from './api/project.api.service';

@Injectable({
    providedIn: 'root'
})
export class ProjectStore {
    private project = new BehaviorSubject<Project | null>(null);

    public project$ = this.project
        .asObservable()
        .pipe(filter((project): project is Project => !!project));

    private readonly projectApi = inject(ProjectApi);

    public load(idProject: number): void {
        // A failure here leaves project$ never emitting; the global error toast
        // tells the user why the page behind the resolver stayed empty.
        this.projectApi.loadOne$(idProject).subscribe(project => this.project.next(project));
    }

    public update(project: Project): Observable<Project> {
        return this.projectApi
            .update$(project)
            .pipe(tap(savedProject => this.project.next(savedProject)));
    }
}
