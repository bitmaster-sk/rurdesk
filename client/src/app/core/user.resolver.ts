import { Injectable, inject } from '@angular/core';
import {
    ActivatedRouteSnapshot,
    RedirectCommand,
    Resolve,
    Router,
    RouterStateSnapshot
} from '@angular/router';
import { combineLatest, Observable, of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { UserService } from '../auth/user.service';
import { TrackerService } from '../shared/tracker/tracker.service';

@Injectable({ providedIn: 'root' })
export class UserResolver implements Resolve<void | RedirectCommand> {
    private readonly sUser = inject(UserService);
    private readonly sTracker = inject(TrackerService);
    private readonly router = inject(Router);

    public resolve(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot
    ): Observable<void | RedirectCommand> {
        return combineLatest([this.sUser.loadUser(), this.sTracker.loadTracker()]).pipe(
            map(() => undefined),
            catchError(() => of(new RedirectCommand(this.router.parseUrl('/login'))))
        );
    }
}
