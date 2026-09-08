import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
    AdminCreateUserReq,
    AdminCreateUserRes,
    AdminUpdateUserReq,
    AdminUser
} from '../model/admin-user.model';
import { RequestContext } from 'src/app/core/request-context';

@Injectable({ providedIn: 'root' })
export class AdminUserApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<AdminUser[]> {
        return this.http.get<AdminUser[]>('/api/private/admin/user');
    }

    public insert$(req: AdminCreateUserReq): Observable<AdminCreateUserRes> {
        return this.http.post<AdminCreateUserRes>('/api/private/admin/user', req, {
            context: RequestContext.disableErrorToast()
        });
    }

    public setAdmin$(idUser: number, isAdmin: boolean): Observable<void> {
        return this.http.patch<void>(`/api/private/admin/user/${idUser}`, { isAdmin });
    }

    public update$(idUser: number, req: AdminUpdateUserReq): Observable<void> {
        return this.http.patch<void>(`/api/private/admin/user/${idUser}`, req, {
            context: RequestContext.disableErrorToast()
        });
    }

    public delete$(idUser: number): Observable<void> {
        return this.http.delete<void>(`/api/private/admin/user/${idUser}`);
    }
}
