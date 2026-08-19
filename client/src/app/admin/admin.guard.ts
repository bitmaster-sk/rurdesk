import { Injectable, inject } from '@angular/core';
import { CanActivate, Router, UrlTree } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { AuthStore } from '../auth/store/auth.store';

/**
 * AdminGuard blocks the admin area for non-admin users. This is UX only — the server
 * enforces access via the AdminOnly middleware. Guards run BEFORE route resolvers,
 * so on a full page load the session user is not in memory yet — the guard fetches
 * it itself instead of waiting on the user stream (which would hang the navigation).
 */
@Injectable({ providedIn: 'root' })
export class AdminGuard implements CanActivate {
    private readonly router = inject(Router);
    private readonly authStore = inject(AuthStore);

    public canActivate(): Observable<boolean | UrlTree> {
        const current = this.authStore.user();
        const user$ = current ? of(current) : this.authStore.loadUser$();
        return user$.pipe(
            map(user => (user?.isAdmin ? true : this.router.parseUrl('/'))),
            catchError(() => of(this.router.parseUrl('/')))
        );
    }
}
