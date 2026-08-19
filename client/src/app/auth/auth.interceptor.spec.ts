import {
    HttpErrorResponse,
    HttpHandler,
    HttpRequest,
    HttpEvent,
    HttpEventType
} from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, of, throwError } from 'rxjs';
import { AuthInterceptor } from './auth.interceptor';
import { AuthTokenStore } from './store/auth-token.store';

function handlerReturning(obs: Observable<HttpEvent<unknown>>): HttpHandler {
    return { handle: vi.fn().mockReturnValue(obs) };
}

function buildInterceptor(tokenStore: AuthTokenStore, router: Router): AuthInterceptor {
    const injector = Injector.create({
        providers: [
            { provide: AuthTokenStore, useValue: tokenStore },
            { provide: Router, useValue: router }
        ]
    });
    return runInInjectionContext(injector, () => new AuthInterceptor());
}

describe('AuthInterceptor', () => {
    it('attaches the local token as the Authorization header', () => {
        const tokenStore = {
            getToken: () => 'my-token',
            clearToken: vi.fn()
        } as unknown as AuthTokenStore;
        const router = {
            navigateByUrl: vi.fn(),
            getCurrentNavigation: () => null
        } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const event: HttpEvent<unknown> = { type: HttpEventType.Sent };
        const next = handlerReturning(of(event));

        interceptor.intercept(new HttpRequest('GET', '/api/x'), next).subscribe();

        const passed = (next.handle as unknown as { mock: { calls: HttpRequest<unknown>[][] } })
            .mock.calls[0][0];
        expect(passed.headers.get('Authorization')).toBe('my-token');
    });

    it('on 401 clears the token, redirects to /login, and rethrows', async () => {
        const clearToken = vi.fn();
        const navigateByUrl = vi.fn();
        const tokenStore = { getToken: () => 't', clearToken } as unknown as AuthTokenStore;
        const router = { navigateByUrl, url: '/project/1/view' } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const next = {
            handle: () => throwError(() => new HttpErrorResponse({ status: 401 }))
        } as unknown as HttpHandler;

        let errored = false;
        interceptor
            .intercept(new HttpRequest('GET', '/x'), next)
            .subscribe({ error: () => (errored = true) });

        expect(clearToken).toHaveBeenCalled();
        expect(errored).toBe(true);
        await Promise.resolve(); // flush queueMicrotask
        expect(navigateByUrl).toHaveBeenCalledWith('/login');
    });

    it('redirects on 401 even mid-navigation (server-invalidated token)', async () => {
        // The old guard skipped the redirect while a navigation was in progress,
        // deferring to a resolver that may not re-check auth. Now it always redirects.
        const navigateByUrl = vi.fn();
        const tokenStore = {
            getToken: () => 't',
            clearToken: vi.fn()
        } as unknown as AuthTokenStore;
        const router = { navigateByUrl, url: '/project/1/view' } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const next = {
            handle: () => throwError(() => new HttpErrorResponse({ status: 401 }))
        } as unknown as HttpHandler;

        interceptor.intercept(new HttpRequest('GET', '/x'), next).subscribe({ error: () => {} });

        await Promise.resolve();
        expect(navigateByUrl).toHaveBeenCalledWith('/login');
    });

    it('does not redirect again when already on /login', async () => {
        const navigateByUrl = vi.fn();
        const tokenStore = {
            getToken: () => 't',
            clearToken: vi.fn()
        } as unknown as AuthTokenStore;
        const router = { navigateByUrl, url: '/login' } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const next = {
            handle: () => throwError(() => new HttpErrorResponse({ status: 401 }))
        } as unknown as HttpHandler;

        interceptor
            .intercept(new HttpRequest('POST', '/api/public/login'), next)
            .subscribe({ error: () => {} });

        await Promise.resolve();
        expect(navigateByUrl).not.toHaveBeenCalled();
    });

    it('does not redirect away from /register (anonymous bootstrap page)', async () => {
        const navigateByUrl = vi.fn();
        const tokenStore = {
            getToken: () => '',
            clearToken: vi.fn()
        } as unknown as AuthTokenStore;
        const router = { navigateByUrl, url: '/register' } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const next = {
            handle: () => throwError(() => new HttpErrorResponse({ status: 401 }))
        } as unknown as HttpHandler;

        interceptor
            .intercept(new HttpRequest('GET', '/api/private/settings'), next)
            .subscribe({ error: () => {} });

        await Promise.resolve();
        expect(navigateByUrl).not.toHaveBeenCalled();
    });

    it('does not touch auth on non-401 errors', () => {
        const clearToken = vi.fn();
        const tokenStore = { getToken: () => 't', clearToken } as unknown as AuthTokenStore;
        const router = { navigateByUrl: vi.fn(), url: '/x' } as unknown as Router;
        const interceptor = buildInterceptor(tokenStore, router);
        const next = {
            handle: () => throwError(() => new HttpErrorResponse({ status: 500 }))
        } as unknown as HttpHandler;

        let errored = false;
        interceptor
            .intercept(new HttpRequest('GET', '/x'), next)
            .subscribe({ error: () => (errored = true) });

        expect(clearToken).not.toHaveBeenCalled();
        expect(errored).toBe(true);
    });
});
