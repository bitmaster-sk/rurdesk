import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, Resolve, RouterStateSnapshot } from '@angular/router';
import { SeverityStore } from './store/severity.store';

@Injectable({ providedIn: 'root' })
export class SeverityResolver implements Resolve<void> {
    private severityStore = inject(SeverityStore);

    public resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): void {
        this.severityStore.load();
    }
}
