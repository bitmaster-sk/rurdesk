import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { User } from '../../auth/model/user.model';
import { Project, CreateProjectReq } from '../model/project.model';

@Injectable({
    providedIn: 'root'
})
export class ProjectApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<Project[]> {
        return this.http.get<Project[]>('/api/private/project');
    }

    public loadOne$(idProject: number): Observable<Project> {
        return this.http.get<Project>(`/api/private/project/${idProject}`);
    }

    public insert$(project: CreateProjectReq): Observable<Project> {
        return this.http.post<Project>('/api/private/project', project);
    }

    public update$(project: Project): Observable<Project> {
        return this.http.patch<Project>('/api/private/project', project);
    }

    public loadMembers$(idProject: number): Observable<User[]> {
        return this.http.get<User[]>(`/api/private/project/${idProject}/members`);
    }
}
