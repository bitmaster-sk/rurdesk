import { Injectable } from '@angular/core';

const KEY = 'Authorization';

@Injectable({ providedIn: 'root' })
export class AuthTokenStore {
    // The session token lives in localStorage rather than an httpOnly cookie
    // because the WebSocket has to read it to authenticate its handshake. A
    // cookie copy existed once and drifted out of sync with this one, breaking
    // reconnects. The cost is that an XSS on the origin can read a session, so
    // the client treats XSS as the boundary to defend.
    public saveToken(token: string): void {
        localStorage.setItem(KEY, token);
    }

    public getToken(): string | null {
        return localStorage.getItem(KEY);
    }

    public clearToken(): void {
        localStorage.removeItem(KEY);
    }

    public hasToken(): boolean {
        return !!localStorage.getItem(KEY);
    }
}
