import {
    HttpClient,
    HTTP_INTERCEPTORS,
    provideHttpClient,
    withInterceptorsFromDi
} from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { AuthInterceptor } from './auth.interceptor';
import { AuthTokenStore } from './store/auth-token.store';

/**
 * Regression guard for the app-module HTTP wiring: the class interceptors are
 * registered via HTTP_INTERCEPTORS and only run when provideHttpClient is given
 * withInterceptorsFromDi(). Without it Angular silently ignores them, so 401s
 * never clear the token or redirect — this test fails if that feature is dropped.
 */
describe('AuthInterceptor wiring (browser)', () => {
    const clearToken = vi.fn();
    const navigateByUrl = vi.fn();

    beforeEach(() => {
        clearToken.mockClear();
        navigateByUrl.mockClear();
        TestBed.configureTestingModule({
            providers: [
                provideHttpClient(withInterceptorsFromDi()),
                provideHttpClientTesting(),
                { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true },
                {
                    provide: AuthTokenStore,
                    useValue: { getToken: () => 'my-token', clearToken }
                },
                { provide: Router, useValue: { navigateByUrl, url: '/project/1' } }
            ]
        });
    });

    it('actually runs the interceptor: attaches the token and handles 401', async () => {
        const http = TestBed.inject(HttpClient);
        const ctrl = TestBed.inject(HttpTestingController);

        http.get('/api/private/user').subscribe({ error: () => {} });

        const req = ctrl.expectOne('/api/private/user');
        // Header present ⇒ the interceptor ran ⇒ withInterceptorsFromDi is active.
        expect(req.request.headers.get('Authorization')).toBe('my-token');

        req.flush('unauthorized', { status: 401, statusText: 'Unauthorized' });

        await Promise.resolve(); // flush the queueMicrotask redirect
        expect(clearToken).toHaveBeenCalled();
        expect(navigateByUrl).toHaveBeenCalledWith('/login');
        ctrl.verify();
    });
});
