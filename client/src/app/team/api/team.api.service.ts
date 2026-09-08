import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Team } from '../model/team.model';

@Injectable({
    providedIn: 'root'
})
export class TeamApi {
    private readonly http = inject(HttpClient);

    /** All teams — any authenticated user (project-member dropdowns, admin screen). */
    public load$(): Observable<Team[]> {
        return this.http.get<Team[]>('/api/private/team');
    }

    /** Teams the current user is a member of (chat menu). */
    public loadMy$(): Observable<Team[]> {
        return this.http.get<Team[]>('/api/private/team/my');
    }
}
