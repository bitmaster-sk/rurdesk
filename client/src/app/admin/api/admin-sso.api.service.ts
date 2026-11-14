import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import {
    InsertSsoMappingReq,
    MoveSsoIdentityReq,
    SaveSsoDraftReq,
    SetSsoEnforcedReq,
    SetSsoScimEnabledReq,
    SsoScimTokenRes,
    SsoGroupMapping,
    SsoIdentity,
    SsoState,
    SsoTestResult,
    StartSsoTestRes,
    UpdateSsoRulesReq
} from '../model/admin-sso.model';

const BASE_URL = '/api/private/admin/sso';

@Injectable({ providedIn: 'root' })
export class AdminSsoApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<SsoState> {
        return this.http.get<SsoState>(BASE_URL, { context: RequestContext.disableErrorToast() });
    }

    public updateDraft$(req: SaveSsoDraftReq): Observable<SsoState> {
        return this.http.put<SsoState>(`${BASE_URL}/draft`, req);
    }

    public startTest$(): Observable<StartSsoTestRes> {
        return this.http.post<StartSsoTestRes>(`${BASE_URL}/test`, {});
    }

    public loadTestResult$(idTest: string): Observable<SsoTestResult> {
        return this.http.get<SsoTestResult>(`${BASE_URL}/test/${encodeURIComponent(idTest)}`);
    }

    public activate$(): Observable<SsoState> {
        return this.http.post<SsoState>(`${BASE_URL}/activate`, {});
    }

    public disable$(): Observable<SsoState> {
        return this.http.post<SsoState>(`${BASE_URL}/disable`, {});
    }

    public setEnforced$(req: SetSsoEnforcedReq): Observable<SsoState> {
        return this.http.put<SsoState>(`${BASE_URL}/enforce`, req);
    }

    public setScimEnabled$(req: SetSsoScimEnabledReq): Observable<SsoState> {
        return this.http.put<SsoState>(`${BASE_URL}/scim`, req);
    }

    public generateScimToken$(): Observable<SsoScimTokenRes> {
        return this.http.post<SsoScimTokenRes>(`${BASE_URL}/scim/token`, {});
    }

    public deleteScimToken$(): Observable<SsoState> {
        return this.http.delete<SsoState>(`${BASE_URL}/scim/token`);
    }

    public regenerateSpCertificate$(): Observable<SsoState> {
        return this.http.post<SsoState>(`${BASE_URL}/sp-certificate`, {});
    }

    public updateRules$(req: UpdateSsoRulesReq): Observable<SsoState> {
        return this.http.put<SsoState>(`${BASE_URL}/settings`, req);
    }

    public loadMappings$(): Observable<SsoGroupMapping[]> {
        return this.http.get<SsoGroupMapping[]>(`${BASE_URL}/mapping`, {
            context: RequestContext.disableErrorToast()
        });
    }

    public insertMapping$(req: InsertSsoMappingReq): Observable<SsoGroupMapping[]> {
        return this.http.post<SsoGroupMapping[]>(`${BASE_URL}/mapping`, req);
    }

    public deleteMapping$(idMapping: number): Observable<SsoGroupMapping[]> {
        return this.http.delete<SsoGroupMapping[]>(`${BASE_URL}/mapping/${idMapping}`);
    }

    public loadIdentities$(idUser: number): Observable<SsoIdentity[]> {
        return this.http.get<SsoIdentity[]>(`${BASE_URL}/identity/${idUser}`, {
            context: RequestContext.disableErrorToast()
        });
    }

    public unlinkIdentity$(idUser: number): Observable<SsoIdentity[]> {
        return this.http.delete<SsoIdentity[]>(`${BASE_URL}/identity/${idUser}`, {
            context: RequestContext.disableErrorToast()
        });
    }

    public moveIdentity$(idUser: number, req: MoveSsoIdentityReq): Observable<SsoIdentity[]> {
        return this.http.post<SsoIdentity[]>(`${BASE_URL}/identity/${idUser}/move`, req, {
            context: RequestContext.disableErrorToast()
        });
    }
}
