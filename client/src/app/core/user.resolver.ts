import { Injectable, inject } from '@angular/core';
import { RedirectCommand, Resolve, Router } from '@angular/router';
import { combineLatest, Observable, of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { AuthStore } from '../auth/store/auth.store';
import { TrackerService } from '../shared/tracker/tracker.service';

@Injectable({ providedIn: 'root' })
export class UserResolver implements Resolve<void | RedirectCommand> {
    private readonly authStore = inject(AuthStore);
    private readonly trackerService = inject(TrackerService);
    private readonly router = inject(Router);

    public resolve(): Observable<void | RedirectCommand> {
        return combineLatest([this.authStore.loadUser$(), this.trackerService.loadTracker$()]).pipe(
            map(() => undefined),
            catchError(() => of(new RedirectCommand(this.router.parseUrl('/login'))))
        );
    }
}
