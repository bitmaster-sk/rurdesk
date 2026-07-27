import { Injectable } from '@angular/core';
import {
    HttpRequest,
    HttpHandler,
    HttpEvent,
    HttpInterceptor,
    HttpErrorResponse
} from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { UserService } from './user.service';
import { catchError } from 'rxjs/operators';
import { Router } from '@angular/router';

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
    constructor(
        private sUser: UserService,
        private router: Router
    ) {}

    intercept(request: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
        const token = this.sUser.getAuthLocal();

        request = request.clone({
            setHeaders: {
                // eslint-disable-next-line @typescript-eslint/naming-convention
                Authorization: token ? token : ''
            }
        });
        return next.handle(request).pipe(
            catchError((e: HttpErrorResponse) => {
                if (e.status === 401) {
                    this.sUser.deleteAuthLocal();
                    // A 401 means the server rejected the token (expired, or invalidated
                    // server-side while the FE still holds it). Always force login — deferred
                    // via microtask to break out of any in-progress navigation/resolver call
                    // stack. Skip on the auth pages themselves (/login, /register): they are
                    // reachable anonymously, so a stray 401 there must not yank the user away.
                    queueMicrotask(() => {
                        const url = this.router.url;
                        if (!url.startsWith('/login') && !url.startsWith('/register')) {
                            this.router.navigateByUrl('/login');
                        }
                    });
                }
                return throwError(() => e);
            })
        );
    }
}
