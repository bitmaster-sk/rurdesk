import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { SeverityStore } from './store/severity.store';

@Injectable({ providedIn: 'root' })
export class SeverityResolver implements Resolve<void> {
    private severityStore = inject(SeverityStore);

    public resolve(): void {
        this.severityStore.load();
    }
}
