import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { StateStore } from './store/state.store';

@Injectable({ providedIn: 'root' })
export class StateResolver implements Resolve<void> {
    private stateStore = inject(StateStore);

    public resolve(): void {
        this.stateStore.load();
    }
}
