import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
    AdminCreateUserReq,
    AdminCreateUserRes,
    AdminUpdateUserReq,
    AdminUser,
    BotApiKey,
    BotGateway,
    CreateBotGatewayReq,
    CreateBotGatewayRes,
    CreateBotKeyRes
} from '../model/admin-user.model';
import { Team } from '../../team/model/team.model';
import { User } from '../../auth/model/user.model';
import { AppSettings, UpdateAppSettingsReq } from '../model/app-settings.model';
import { silentErrors } from 'src/app/core/http-error-context';

@Injectable({ providedIn: 'root' })
export class AdminApi {
    private readonly http = inject(HttpClient);

    public listUsers$(): Observable<AdminUser[]> {
        return this.http.get<AdminUser[]>('/api/private/admin/user');
    }

    public createUser$(req: AdminCreateUserReq): Observable<AdminCreateUserRes> {
        return this.http.post<AdminCreateUserRes>('/api/private/admin/user', req, {
            context: silentErrors()
        });
    }

    public setAdmin$(idUser: number, isAdmin: boolean): Observable<void> {
        return this.http.patch<void>(`/api/private/admin/user/${idUser}`, { isAdmin });
    }

    public updateUser$(idUser: number, req: AdminUpdateUserReq): Observable<void> {
        return this.http.patch<void>(`/api/private/admin/user/${idUser}`, req, {
            context: silentErrors()
        });
    }

    public deleteUser$(idUser: number): Observable<void> {
        return this.http.delete<void>(`/api/private/admin/user/${idUser}`);
    }

    public getBotKey$(idUser: number): Observable<BotApiKey | null> {
        return this.http.get<BotApiKey | null>(`/api/private/admin/user/${idUser}/api-key`, {
            context: silentErrors()
        });
    }

    public createBotKey$(idUser: number, name: string): Observable<CreateBotKeyRes> {
        return this.http.post<CreateBotKeyRes>(
            `/api/private/admin/user/${idUser}/api-key`,
            {
                name
            },
            { context: silentErrors() }
        );
    }

    public regenerateBotKey$(idUser: number): Observable<CreateBotKeyRes> {
        return this.http.post<CreateBotKeyRes>(
            `/api/private/admin/user/${idUser}/api-key/token`,
            {},
            { context: silentErrors() }
        );
    }

    public deleteBotKey$(idUser: number): Observable<void> {
        return this.http.delete<void>(`/api/private/admin/user/${idUser}/api-key`, {
            context: silentErrors()
        });
    }

    public getBotGateway$(idUser: number): Observable<BotGateway | null> {
        return this.http.get<BotGateway | null>(`/api/private/admin/user/${idUser}/gateway`, {
            context: silentErrors()
        });
    }

    public createBotGateway$(
        idUser: number,
        req: CreateBotGatewayReq
    ): Observable<CreateBotGatewayRes> {
        return this.http.post<CreateBotGatewayRes>(
            `/api/private/admin/user/${idUser}/gateway`,
            req,
            { context: silentErrors() }
        );
    }

    public updateBotGatewayUrl$(idUser: number, req: CreateBotGatewayReq): Observable<BotGateway> {
        return this.http.patch<BotGateway>(`/api/private/admin/user/${idUser}/gateway`, req);
    }

    public regenerateGatewayToken$(idUser: number): Observable<CreateBotGatewayRes> {
        return this.http.post<CreateBotGatewayRes>(
            `/api/private/admin/user/${idUser}/gateway/token`,
            {},
            { context: silentErrors() }
        );
    }

    public deleteBotGateway$(idUser: number): Observable<void> {
        return this.http.delete<void>(`/api/private/admin/user/${idUser}/gateway`, {
            context: silentErrors()
        });
    }

    public createTeam$(team: { name: string; color: string }): Observable<Team> {
        return this.http.post<Team>('/api/private/admin/team', team);
    }

    public updateTeam$(team: Team): Observable<Team> {
        return this.http.patch<Team>('/api/private/admin/team', team);
    }

    public deleteTeam$(idTeam: number): Observable<void> {
        const params = new HttpParams({ fromObject: { idTeam: idTeam.toString() } });
        return this.http.delete<void>('/api/private/admin/team', { params });
    }

    public listTeamMembers$(idTeam: number): Observable<User[]> {
        return this.http.get<User[]>(`/api/private/admin/team/${idTeam}/member`);
    }

    public addTeamMember$(idTeam: number, idUser: number): Observable<void> {
        return this.http.post<void>('/api/private/admin/team/member', { idTeam, idUser });
    }

    public removeTeamMember$(idTeam: number, idUser: number): Observable<void> {
        let params = new HttpParams();
        params = params.append('idTeam', idTeam.toString());
        params = params.append('idUser', idUser.toString());
        return this.http.delete<void>('/api/private/admin/team/member', { params });
    }

    public getSettings$(): Observable<AppSettings> {
        // readable by any authenticated user
        return this.http.get<AppSettings>('/api/private/settings');
    }

    public updateSettings$(req: UpdateAppSettingsReq): Observable<AppSettings> {
        // admin-only
        return this.http.patch<AppSettings>('/api/private/admin/settings', req);
    }
}
