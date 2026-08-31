import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { CreateUserApiKeyReq, CreatedUserApiKey, UserApiKey } from '../model/user-api-key.model';

const BASE_URL = '/api/private/user/api-key';

@Injectable({ providedIn: 'root' })
export class UserApiKeyApi {
    private readonly http = inject(HttpClient);

    public load$(): Observable<UserApiKey[]> {
        return this.http.get<UserApiKey[]>(BASE_URL);
    }

    public insert$(req: CreateUserApiKeyReq): Observable<CreatedUserApiKey> {
        return this.http.post<CreatedUserApiKey>(BASE_URL, req);
    }

    public regenerate$(idApiKey: number): Observable<CreatedUserApiKey> {
        return this.http.post<CreatedUserApiKey>(`${BASE_URL}/${idApiKey}/token`, {});
    }

    public revoke$(idApiKey: number): Observable<void> {
        return this.http.delete<void>(`${BASE_URL}/${idApiKey}`);
    }
}
