import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ProjectSkill, UpdateProjectSkillReq } from '../model/project-skill.model';

@Injectable({ providedIn: 'root' })
export class ProjectSkillApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number): Observable<ProjectSkill[]> {
        return this.http.get<ProjectSkill[]>(`/api/private/project/${idProject}/skills`);
    }

    public replace$(
        idProject: number,
        entries: UpdateProjectSkillReq[]
    ): Observable<ProjectSkill[]> {
        return this.http.put<ProjectSkill[]>(`/api/private/project/${idProject}/skills`, entries);
    }
}
