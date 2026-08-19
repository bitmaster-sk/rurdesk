import { HTTP_INTERCEPTORS, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { TranslateService, provideTranslateService } from '@ngx-translate/core';
import { provideTranslateHttpLoader } from '@ngx-translate/http-loader';
import { AuthInterceptor } from './auth/auth.interceptor';
import { AuthTokenStore } from './auth/store/auth-token.store';
import { ToastNotificationService } from './core/toast-notification.service';
import { ErrorInterceptor } from './core/error.interceptor';

/**
 * Bootstrap wiring guard: withInterceptorsFromDi() makes ErrorInterceptor part of
 * the HttpClient chain, and ErrorInterceptor injects TranslateService. If the
 * translate loader also used that HttpClient (useHttpBackend:false) the loader
 * request would re-enter ErrorInterceptor → TranslateService mid-construction —
 * a DI cycle that silently stops translations from loading. Loading via
 * HttpBackend (useHttpBackend:true) breaks the cycle. This test proves the real
 * interceptors + TranslateService coexist and translations actually load.
 */
describe('HttpClient + interceptors + TranslateService wiring (browser)', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [
                provideHttpClient(withInterceptorsFromDi()),
                provideHttpClientTesting(),
                { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true },
                { provide: HTTP_INTERCEPTORS, useClass: ErrorInterceptor, multi: true },
                {
                    provide: AuthTokenStore,
                    useValue: { getToken: () => null, clearToken: () => {} }
                },
                { provide: Router, useValue: { navigateByUrl: () => {}, url: '/login' } },
                { provide: ToastNotificationService, useValue: { showError: () => {} } },
                provideTranslateService({
                    fallbackLang: 'en',
                    loader: provideTranslateHttpLoader({
                        prefix: '../assets/i18n/',
                        suffix: '.json',
                        enforceLoading: true,
                        useHttpBackend: true
                    })
                })
            ]
        });
    });

    it('loads translations without a DI cycle', () => {
        const translate = TestBed.inject(TranslateService);
        const ctrl = TestBed.inject(HttpTestingController);

        let loaded: string | undefined;
        translate.use('en').subscribe();
        translate.get('GREETING').subscribe(v => (loaded = v));

        // The loader fetched the JSON (via HttpBackend); satisfy it.
        const req = ctrl.expectOne(r => r.url.includes('en.json'));
        req.flush({ GREETING: 'Hello' });

        expect(loaded).toBe('Hello');
        ctrl.verify();
    });
});
