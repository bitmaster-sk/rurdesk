import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { RequestContext } from 'src/app/core/request-context';
import { AuthMethods } from '../model/auth-methods.model';
import { Register } from '../model/register.model';
import { SsoExchangeRes } from '../model/sso-exchange.model';
import { User } from '../model/user.model';

@Injectable({ providedIn: 'root' })
export class AuthApi {
    private readonly http = inject(HttpClient);

    public login$(
        email: string,
        password: string,
        hasExtendedSessionLifetime: boolean
    ): Observable<string> {
        return this.http
            .post<{ token: string }>('/api/public/login', {
                email,
                password,
                hasExtendedSessionLifetime
            })
            .pipe(map(res => res.token));
    }

    public logout$(): Observable<void> {
        return this.http.delete<void>('/api/private/logout');
    }

    public register$(register: Register): Observable<void> {
        return this.http.post<void>('/api/public/register', register, {
            context: RequestContext.disableErrorToast()
        });
    }

    public changePassword$(currentPassword: string, newPassword: string): Observable<void> {
        return this.http.put<void>(
            '/api/private/user/password',
            { currentPassword, newPassword },
            { context: RequestContext.disableErrorToast() }
        );
    }

    public loadUser$(): Observable<User> {
        return this.http.get<User>('/api/private/user');
    }

    public updateUser$(name: string, colorAvatarBg?: string): Observable<User> {
        return this.http.patch<User>(
            '/api/private/user',
            { name, colorAvatarBg },
            { context: RequestContext.disableErrorToast() }
        );
    }

    public loadMethods$(): Observable<AuthMethods> {
        return this.http.get<AuthMethods>('/api/public/auth/methods', {
            context: RequestContext.disableErrorToast()
        });
    }

    public exchangeSso$(code: string): Observable<SsoExchangeRes> {
        return this.http.post<SsoExchangeRes>(
            '/api/public/sso/exchange',
            { code },
            { context: RequestContext.disableErrorToast() }
        );
    }
}
