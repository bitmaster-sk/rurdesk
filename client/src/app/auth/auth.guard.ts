import { Injectable, inject } from '@angular/core';
import { Router, CanActivate, UrlTree } from '@angular/router';
import { Observable } from 'rxjs';
import { UserService } from './user.service';

@Injectable({
    providedIn: 'root'
})
export class AuthGuard implements CanActivate {
    constructor(
        private router: Router,
        private sUser: UserService
    ) {}

    public canActivate(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot
    ): Observable<boolean | UrlTree> | Promise<boolean | UrlTree> | boolean | UrlTree {
        if (this.sUser.hasAuthLocal()) {
            return true;
        }
        this.router.navigate(['/login']);
        return false;
    }
}
