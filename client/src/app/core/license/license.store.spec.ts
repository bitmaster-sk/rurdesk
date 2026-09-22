import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { LicenseApi } from './license.api.service';
import { LicenseStore } from './license.store';
import { LicenseSeverity, type LicenseState } from './model/license.model';

function build(getReturn?: LicenseState) {
    const load$ = vi.fn().mockReturnValue(of(getReturn));
    const injector = Injector.create({
        providers: [{ provide: LicenseApi, useValue: { load$ } }]
    });
    const store = runInInjectionContext(injector, () => new LicenseStore());
    return { store, load$ };
}

describe('LicenseStore', () => {
    it('warns about nothing before load', () => {
        const { store } = build();

        expect(store.state().severity).toBe(LicenseSeverity.None);
        expect(store.state().expiresAt).toBeNull();
    });

    it('exposes the loaded expiry state', () => {
        const { store } = build({
            severity: LicenseSeverity.Error,
            daysRemaining: 6,
            expiresAt: '2026-09-21T00:00:00Z',
            isDismissible: false
        });

        store.load();

        expect(store.state().severity).toBe(LicenseSeverity.Error);
        expect(store.state().daysRemaining).toBe(6);
        expect(store.state().isDismissible).toBe(false);
    });

    it('stays quiet when the instance has no licence', () => {
        const { store } = build({
            severity: LicenseSeverity.None,
            daysRemaining: 0,
            expiresAt: null,
            isDismissible: false
        });

        store.load();

        expect(store.state().severity).toBe(LicenseSeverity.None);
    });
});
