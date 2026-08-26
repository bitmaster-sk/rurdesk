import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { CreateSkillReq, Skill, UpdateSkillReq } from '../model/skill.model';

@Injectable({ providedIn: 'root' })
export class SkillApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<Skill[]> {
        return this.http.get<Skill[]>('/api/private/skills');
    }

    public create$(body: CreateSkillReq): Observable<Skill> {
        return this.http.post<Skill>('/api/private/admin/skills', body);
    }

    public update$(idSkill: number, body: UpdateSkillReq): Observable<Skill> {
        return this.http.patch<Skill>(`/api/private/admin/skills/${idSkill}`, body);
    }

    public delete$(idSkill: number): Observable<void> {
        return this.http.delete<void>(`/api/private/admin/skills/${idSkill}`);
    }

    public restore$(idSkill: number): Observable<Skill> {
        return this.http.post<Skill>(`/api/private/admin/skills/${idSkill}/restore`, {});
    }
}
