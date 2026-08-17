import { Injectable, inject } from '@angular/core';
import {
    HttpRequest,
    HttpHandler,
    HttpEvent,
    HttpInterceptor,
    HttpErrorResponse
} from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, switchMap } from 'rxjs/operators';
import { I18nService } from '../shared/i18n/i18n.service';
import { ToastNotificationService } from './toast-notification.service';
import { SILENCE_ERROR_TOAST } from './http-error-context';
import { ApiError } from '../shared/model/api-error.model';

@Injectable()
export class ErrorInterceptor implements HttpInterceptor {
    private readonly i18n = inject(I18nService);
    private readonly toast = inject(ToastNotificationService);

    public intercept(req: HttpRequest<unknown>, next: HttpHandler): Observable<HttpEvent<unknown>> {
        return next.handle(req).pipe(
            catchError((error: HttpErrorResponse) => {
                // The API renders a uniform { code, message, translateKey } body for every
                // failure (see the backend ErrorRenderer middleware), so notifying here
                // reaches every caller — most subscribe without an error branch and would
                // otherwise fail silently. Callers that expect the failure, or show a more
                // specific message, opt out with SILENCE_ERROR_TOAST. The message is still
                // attached to the re-thrown error either way.
                const notify = !req.context.get(SILENCE_ERROR_TOAST) && this.shouldNotify(error);
                const key = error.error?.translateKey;

                if (key) {
                    return this.i18n.get$(key).pipe(
                        switchMap(msg => {
                            if (notify) {
                                this.toast.showError(key);
                            }
                            return throwError(() => ({ ...error, message: msg }));
                        })
                    );
                }
                if (notify) {
                    this.toast.showError(this.fallbackKey(error));
                }
                return throwError(() => error);
            })
        );
    }

    // 401 already clears the session and redirects to /login; a toast on top of a
    // redirect is noise. Status 0 means the request never reached the server —
    // offline, or cancelled by navigating away.
    private shouldNotify(error: HttpErrorResponse): boolean {
        return error.status !== 401 && error.status !== 0;
    }

    private fallbackKey(error: HttpErrorResponse): string {
        return error.status === 503 ? 'error.ai_unavailable' : 'error.internal';
    }
}
