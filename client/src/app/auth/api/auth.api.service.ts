import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { silentErrors } from 'src/app/core/http-error-context';
import { Register } from '../model/register.model';
import { User } from '../model/user.model';

@Injectable({ providedIn: 'root' })
export class AuthApi {
    private readonly http = inject(HttpClient);

    public login$(email: string, password: string): Observable<string> {
        return this.http
            .post<{ token: string }>('/api/public/login', { email, password })
            .pipe(map(res => res.token));
    }

    public logout$(): Observable<void> {
        return this.http.delete<void>('/api/private/logout');
    }

    public register$(register: Register): Observable<void> {
        return this.http.post<void>('/api/public/register', register);
    }

    public changePassword$(currentPassword: string, newPassword: string): Observable<void> {
        return this.http.put<void>(
            '/api/private/user/password',
            { currentPassword, newPassword },
            { context: silentErrors() }
        );
    }

    public loadUser$(): Observable<User> {
        return this.http.get<User>('/api/private/user');
    }

    public updateUser$(name: string, colorAvatarBg?: string): Observable<User> {
        return this.http.patch<User>(
            '/api/private/user',
            { name, colorAvatarBg },
            { context: silentErrors() }
        );
    }
}
