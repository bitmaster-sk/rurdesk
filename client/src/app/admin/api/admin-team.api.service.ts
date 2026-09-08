import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Team } from '../../team/model/team.model';
import { User } from '../../auth/model/user.model';

@Injectable({ providedIn: 'root' })
export class AdminTeamApi {
    private readonly http = inject(HttpClient);

    public insert$(team: { name: string; color: string }): Observable<Team> {
        return this.http.post<Team>('/api/private/admin/team', team);
    }

    public update$(team: Team): Observable<Team> {
        return this.http.patch<Team>('/api/private/admin/team', team);
    }

    public delete$(idTeam: number): Observable<void> {
        const params = new HttpParams({ fromObject: { idTeam: idTeam.toString() } });
        return this.http.delete<void>('/api/private/admin/team', { params });
    }

    public loadMembers$(idTeam: number): Observable<User[]> {
        return this.http.get<User[]>(`/api/private/admin/team/${idTeam}/member`);
    }

    public insertMember$(idTeam: number, idUser: number): Observable<void> {
        return this.http.post<void>('/api/private/admin/team/member', { idTeam, idUser });
    }

    public deleteMember$(idTeam: number, idUser: number): Observable<void> {
        let params = new HttpParams();
        params = params.append('idTeam', idTeam.toString());
        params = params.append('idUser', idUser.toString());
        return this.http.delete<void>('/api/private/admin/team/member', { params });
    }
}
