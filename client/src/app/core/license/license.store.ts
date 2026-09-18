import { Injectable, inject, signal } from '@angular/core';
import { LicenseApi } from './license.api.service';
import { LicenseSeverity, LicenseState } from './model/license.model';

// Safe state before the first response: a free instance is never warned.
const FALLBACK: LicenseState = {
    severity: LicenseSeverity.None,
    daysRemaining: 0,
    expiresAt: null,
    isDismissible: false
};

@Injectable({ providedIn: 'root' })
export class LicenseStore {
    private readonly api = inject(LicenseApi);

    private readonly license = signal<LicenseState>(FALLBACK);

    public readonly state = (): LicenseState => this.license();

    public load(): void {
        this.api.load$().subscribe(state => this.license.set(state));
    }
}
