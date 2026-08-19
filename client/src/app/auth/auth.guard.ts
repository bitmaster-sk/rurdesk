import { Injectable, inject } from '@angular/core';
import { Router, CanActivate, UrlTree } from '@angular/router';
import { Observable } from 'rxjs';
import { AuthTokenStore } from './store/auth-token.store';

@Injectable({
    providedIn: 'root'
})
export class AuthGuard implements CanActivate {
    private readonly router = inject(Router);
    private readonly tokenStore = inject(AuthTokenStore);

    public canActivate():
        Observable<boolean | UrlTree> | Promise<boolean | UrlTree> | boolean | UrlTree {
        if (this.tokenStore.hasToken()) {
            return true;
        }
        void this.router.navigate(['/login']);
        return false;
    }
}
