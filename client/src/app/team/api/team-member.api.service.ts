import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { User } from '../../auth/model/user.model';

@Injectable({ providedIn: 'root' })
export class TeamMemberApi {
    private readonly http = inject(HttpClient);

    public list$(idTeam: number): Observable<User[]> {
        return this.http.get<User[]>(`/api/private/team/${idTeam}/members`);
    }
}
