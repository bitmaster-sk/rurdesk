import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { User } from '../auth/model/user.model';
import { Project, ProjectInsert } from './model/project.model';

@Injectable({
    providedIn: 'root'
})
export class ProjectService {
    private readonly http = inject(HttpClient);

    public loadProjects(): Observable<Project[]> {
        return this.http.get<Project[]>('/api/private/project');
    }

    public loadProject(idProject: number): Observable<Project> {
        return this.http.get<Project>(`/api/private/project/${idProject}`);
    }

    public insertProject(project: ProjectInsert): Observable<Project> {
        return this.http.post<Project>('/api/private/project', project);
    }

    public updateProject(project: Project): Observable<Project> {
        return this.http.patch<Project>('/api/private/project', project);
    }

    public loadMembers(idProject: number): Observable<User[]> {
        return this.http.get<User[]>(`/api/private/project/${idProject}/members`);
    }
}
