import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
    CreateReleaseReq,
    DeleteReleaseReq,
    Release,
    ReleaseDraft,
    ReleaseView,
    StartReleaseAgentReq,
    UpdateReleaseReq
} from '../model/release.model';

@Injectable({ providedIn: 'root' })
export class ReleaseApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number): Observable<ReleaseView[]> {
        return this.http.get<ReleaseView[]>(`/api/private/project/${idProject}/release`);
    }

    public loadOne$(idProject: number, idRelease: number): Observable<ReleaseView> {
        return this.http.get<ReleaseView>(`/api/private/project/${idProject}/release/${idRelease}`);
    }

    public loadDraft$(idProject: number, idTag: number): Observable<ReleaseDraft> {
        return this.http.get<ReleaseDraft>(`/api/private/project/${idProject}/release/draft`, {
            params: { idTag }
        });
    }

    public insert$(idProject: number, body: CreateReleaseReq): Observable<ReleaseView> {
        return this.http.post<ReleaseView>(`/api/private/project/${idProject}/release`, body);
    }

    public update$(
        idProject: number,
        idRelease: number,
        body: UpdateReleaseReq
    ): Observable<ReleaseView> {
        return this.http.put<ReleaseView>(
            `/api/private/project/${idProject}/release/${idRelease}`,
            body
        );
    }

    public startAgent$(
        idProject: number,
        idRelease: number,
        body: StartReleaseAgentReq
    ): Observable<ReleaseView> {
        return this.http.post<ReleaseView>(
            `/api/private/project/${idProject}/release/${idRelease}/agent`,
            body
        );
    }

    public publish$(idProject: number, idRelease: number): Observable<ReleaseView> {
        return this.http.post<ReleaseView>(
            `/api/private/project/${idProject}/release/${idRelease}/publish`,
            {}
        );
    }

    public updateNotes$(
        idProject: number,
        idRelease: number,
        notes: string
    ): Observable<ReleaseView> {
        return this.http.put<ReleaseView>(
            `/api/private/project/${idProject}/release/${idRelease}/notes`,
            { notes }
        );
    }

    public delete$(
        idProject: number,
        idRelease: number,
        body: DeleteReleaseReq
    ): Observable<Release> {
        return this.http.delete<Release>(`/api/private/project/${idProject}/release/${idRelease}`, {
            body
        });
    }
}
