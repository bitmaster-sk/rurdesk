import { HttpErrorResponse, HttpHandler, HttpRequest } from '@angular/common/http';
import type { TranslateService } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ErrorInterceptor } from './error.interceptor';
import { silentErrors } from './http-error-context';
import type { ToastNotificationService } from './toast-notification.service';

function build() {
    const showError = vi.fn();
    const translate = { get: (k: string) => of(`t:${k}`) } as unknown as TranslateService;
    const toast = { showError } as unknown as ToastNotificationService;
    return { interceptor: new ErrorInterceptor(translate, toast), showError };
}

function run(interceptor: ErrorInterceptor, error: HttpErrorResponse, silent = false) {
    const next = { handle: () => throwError(() => error) } as unknown as HttpHandler;
    const req = new HttpRequest('GET', '/x', silent ? { context: silentErrors() } : {});
    let caught: unknown;
    interceptor.intercept(req, next).subscribe({ error: e => (caught = e) });
    return caught;
}

describe('ErrorInterceptor', () => {
    it('translates the message and toasts the key the API sent', () => {
        const { interceptor, showError } = build();
        const caught = run(
            interceptor,
            new HttpErrorResponse({ status: 503, error: { translateKey: 'error.x' } })
        );
        expect(showError).toHaveBeenCalledWith('error.x');
        expect((caught as { message: string }).message).toBe('t:error.x');
    });

    // Most callers subscribe without an error branch, so the interceptor is the
    // only thing standing between a failed request and a silent no-op.
    it('toasts an error carrying no translateKey', () => {
        const { interceptor, showError } = build();
        run(interceptor, new HttpErrorResponse({ status: 500 }));
        expect(showError).toHaveBeenCalledWith('error.internal');
    });

    it('toasts ai_unavailable on a 503 without a translateKey', () => {
        const { interceptor, showError } = build();
        run(interceptor, new HttpErrorResponse({ status: 503 }));
        expect(showError).toHaveBeenCalledWith('error.ai_unavailable');
    });

    it('re-throws the original error when there is no translateKey', () => {
        const { interceptor } = build();
        const err = new HttpErrorResponse({ status: 500 });
        expect(run(interceptor, err)).toBe(err);
    });

    // 401 redirects to /login; a toast on top of a redirect is noise.
    it('stays quiet on 401', () => {
        const { interceptor, showError } = build();
        run(
            interceptor,
            new HttpErrorResponse({ status: 401, error: { translateKey: 'error.unauthorized' } })
        );
        expect(showError).not.toHaveBeenCalled();
    });

    // Status 0 means the request never reached the server — offline, or cancelled
    // by navigating away, neither of which is worth interrupting the user for.
    it('stays quiet when the request never reached the server', () => {
        const { interceptor, showError } = build();
        run(interceptor, new HttpErrorResponse({ status: 0 }));
        expect(showError).not.toHaveBeenCalled();
    });

    it('stays quiet for a caller that opted out, but still re-throws', () => {
        const { interceptor, showError } = build();
        const caught = run(
            interceptor,
            new HttpErrorResponse({ status: 404, error: { translateKey: 'error.not_found' } }),
            true
        );
        expect(showError).not.toHaveBeenCalled();
        expect((caught as { message: string }).message).toBe('t:error.not_found');
    });
});
