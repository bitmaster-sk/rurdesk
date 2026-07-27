import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Role } from '../../shared/constants/role.enum';
import { ProjectMembersRes } from '../model/project-members.model';

@Injectable({ providedIn: 'root' })
export class ProjectMemberApi {
    private readonly http = inject(HttpClient);

    public getUserRole(idProject: number): Observable<{ role: Role }> {
        return this.http.get<{ role: Role }>(`/api/private/project/${idProject}/user-role`);
    }

    public getMembers(idProject: number): Observable<ProjectMembersRes> {
        return this.http.get<ProjectMembersRes>(`/api/private/project/${idProject}/member`);
    }

    public addUser(idProject: number, idUser: number, role: Role): Observable<void> {
        return this.http.post<void>(`/api/private/project/${idProject}/member/user`, {
            idUser,
            role
        });
    }

    public updateUserRole(idProject: number, idUser: number, role: Role): Observable<void> {
        return this.http.patch<void>(`/api/private/project/${idProject}/member/user/${idUser}`, {
            role
        });
    }

    public removeUser(idProject: number, idUser: number): Observable<void> {
        return this.http.delete<void>(`/api/private/project/${idProject}/member/user/${idUser}`);
    }

    public addTeam(idProject: number, idTeam: number, role: Role): Observable<void> {
        return this.http.post<void>(`/api/private/project/${idProject}/member/team`, {
            idTeam,
            role
        });
    }

    public updateTeamRole(idProject: number, idTeam: number, role: Role): Observable<void> {
        return this.http.patch<void>(`/api/private/project/${idProject}/member/team/${idTeam}`, {
            role
        });
    }

    public removeTeam(idProject: number, idTeam: number): Observable<void> {
        return this.http.delete<void>(`/api/private/project/${idProject}/member/team/${idTeam}`);
    }
}
