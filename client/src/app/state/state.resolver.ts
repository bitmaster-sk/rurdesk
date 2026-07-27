import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, Resolve, RouterStateSnapshot } from '@angular/router';
import { StateStore } from './store/state.store';

@Injectable({ providedIn: 'root' })
export class StateResolver implements Resolve<void> {
    private stateStore = inject(StateStore);

    public resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): void {
        this.stateStore.load();
    }
}
