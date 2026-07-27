import { User } from './model/user.model';
import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { filter, map, tap } from 'rxjs/operators';
import { Register } from './model/register.model';
import { silentErrors } from 'src/app/core/http-error-context';

@Injectable({
    providedIn: 'root'
})
export class UserService {
    public user: BehaviorSubject<User> = new BehaviorSubject<User>(null);

    public user$ = this.user.pipe(filter(u => u !== null));

    constructor(private http: HttpClient) {}

    public login(email: string, password: string): Observable<string> {
        return this.http
            .post<{ token: string }>('/api/public/login', { email, password })
            .pipe(map(res => res.token));
    }

    public logout(): Observable<void> {
        return this.http.delete<void>('/api/private/logout');
    }

    public register(register: Register): Observable<void> {
        return this.http.post<void>('/api/public/register', register);
    }

    public loadUser(): Observable<User> {
        return this.http.get<User>('/api/private/user').pipe(tap(u => this.user.next(u)));
    }

    public updateUser(name: string, colorAvatarBg?: string): Observable<User> {
        return this.http
            .patch<User>('/api/private/user', { name, colorAvatarBg }, { context: silentErrors() })
            .pipe(tap(u => this.user.next(u)));
    }

    public changePassword$(currentPassword: string, newPassword: string): Observable<void> {
        return this.http.put<void>(
            '/api/private/user/password',
            { currentPassword, newPassword },
            { context: silentErrors() }
        );
    }

    // The session token lives in localStorage rather than an httpOnly cookie
    // because the WebSocket has to read it to authenticate its handshake. A
    // cookie copy existed once and drifted out of sync with this one, breaking
    // reconnects. The cost is that an XSS on the origin can read a session, so
    // the client treats XSS as the boundary to defend.
    public saveAuthLocal(token: string): void {
        localStorage.setItem('Authorization', token);
    }

    public deleteAuthLocal(): void {
        localStorage.removeItem('Authorization');
    }

    public getAuthLocal(): string {
        return localStorage.getItem('Authorization');
    }

    public hasAuthLocal(): boolean {
        return !!localStorage.getItem('Authorization');
    }
}
