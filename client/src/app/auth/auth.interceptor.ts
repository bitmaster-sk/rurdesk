import { Injectable, inject } from '@angular/core';
import {
    HttpRequest,
    HttpHandler,
    HttpEvent,
    HttpInterceptor,
    HttpErrorResponse
} from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { AuthTokenStore } from './store/auth-token.store';
import { catchError } from 'rxjs/operators';
import { Router } from '@angular/router';
import { environment } from '../../environments/environment';

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
    private readonly tokenStore = inject(AuthTokenStore);
    private readonly router = inject(Router);

    /**
     * Origins that are allowed to receive the JWT `Authorization` header.
     * Built once per interceptor instance:
     *   - the app's own origin (always)
     *   - `environment.mcpPublicBaseUrl` when configured
     */
    private readonly allowedOrigins: ReadonlySet<string> = (() => {
        const origins = new Set<string>();
        if (typeof window !== 'undefined') {
            origins.add(window.location.origin);
        }
        if (environment.mcpPublicBaseUrl) {
            origins.add(environment.mcpPublicBaseUrl);
        }
        return origins;
    })();

    /**
     * Returns `true` when the request URL targets an allowed origin.
     * Relative URLs are always allowed (they resolve to the own origin).
     * Malformed URLs fail-closed (no header attached).
     */
    private isAllowedOrigin(url: string): boolean {
        // Relative URL — no scheme/host, resolves to own origin.
        if (!/^https?:\/\//i.test(url)) {
            return true;
        }
        try {
            return this.allowedOrigins.has(new URL(url).origin);
        } catch {
            return false;
        }
    }

    public intercept(
        request: HttpRequest<unknown>,
        next: HttpHandler
    ): Observable<HttpEvent<unknown>> {
        const token = this.tokenStore.getToken();

        if (token && this.isAllowedOrigin(request.url)) {
            request = request.clone({
                setHeaders: {
                    Authorization: token
                }
            });
        }
        return next.handle(request).pipe(
            catchError((e: HttpErrorResponse) => {
                if (e.status === 401) {
                    this.tokenStore.clearToken();
                    // A 401 means the server rejected the token (expired, or invalidated
                    // server-side while the FE still holds it). Always force login — deferred
                    // via microtask to break out of any in-progress navigation/resolver call
                    // stack. Skip on the auth pages themselves (/login, /register): they are
                    // reachable anonymously, so a stray 401 there must not yank the user away.
                    queueMicrotask(() => {
                        const url = this.router.url;
                        if (!url.startsWith('/login') && !url.startsWith('/register')) {
                            void this.router.navigateByUrl('/login');
                        }
                    });
                }
                return throwError(() => e);
            })
        );
    }
}
