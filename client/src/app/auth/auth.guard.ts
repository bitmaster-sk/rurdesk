import { Injectable, inject } from '@angular/core';
import { Router, CanActivate, UrlTree } from '@angular/router';
import { Observable } from 'rxjs';
import { UserService } from './user.service';

@Injectable({
    providedIn: 'root'
})
export class AuthGuard implements CanActivate {
    private readonly router = inject(Router);
    private readonly sUser = inject(UserService);

    public canActivate():
        Observable<boolean | UrlTree> | Promise<boolean | UrlTree> | boolean | UrlTree {
        if (this.sUser.hasAuthLocal()) {
            return true;
        }
        void this.router.navigate(['/login']);
        return false;
    }
}
