import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { format, subDays } from 'date-fns';
import { TablerIconStub } from 'src/testing/stubs';
import { LicenseStore } from 'src/app/core/license/license.store';
import { LicenseSeverity, LicenseState } from 'src/app/core/license/model/license.model';
import { LicenseBannerComponent } from './license-banner.component';

const DISMISSED_KEY = 'rurdesk.license-banner-dismissed-on';

const QUIET: LicenseState = {
    severity: LicenseSeverity.None,
    daysRemaining: 0,
    expiresAt: null,
    isDismissible: false
};

function warningOf(severity: LicenseSeverity, daysRemaining: number): LicenseState {
    return {
        severity,
        daysRemaining,
        expiresAt: '2026-09-29T00:00:00Z',
        isDismissible: severity === LicenseSeverity.Info || severity === LicenseSeverity.Warning
    };
}

async function render(state: LicenseState) {
    await TestBed.configureTestingModule({
        imports: [TranslateModule.forRoot(), TablerIconStub],
        declarations: [LicenseBannerComponent],
        providers: [{ provide: LicenseStore, useValue: { state: () => state } }]
    }).compileComponents();

    const fixture = TestBed.createComponent(LicenseBannerComponent);
    fixture.detectChanges();
    return fixture;
}

function bannerOf(fixture: Awaited<ReturnType<typeof render>>): HTMLElement | null {
    return fixture.nativeElement.querySelector('[data-testid="license-banner"]');
}

describe('LicenseBannerComponent', () => {
    beforeEach(() => {
        localStorage.clear();
        TestBed.resetTestingModule();
    });

    it('renders nothing when there is no warning', async () => {
        const fixture = await render(QUIET);

        expect(bannerOf(fixture)).toBeNull();
    });

    it('carries the severity as a class so the step is visible', async () => {
        const fixture = await render(warningOf(LicenseSeverity.Warning, 14));

        expect(bannerOf(fixture)?.classList.contains('warning')).toBe(true);
    });

    it('offers a dismiss control on the calm steps', async () => {
        const fixture = await render(warningOf(LicenseSeverity.Info, 27));

        expect(
            fixture.nativeElement.querySelector('[data-testid="license-banner-dismiss"]')
        ).not.toBeNull();
    });

    it('cannot be dismissed once expiry is close', async () => {
        const fixture = await render(warningOf(LicenseSeverity.Error, 6));

        expect(bannerOf(fixture)).not.toBeNull();
        expect(
            fixture.nativeElement.querySelector('[data-testid="license-banner-dismiss"]')
        ).toBeNull();
    });

    it('cannot be dismissed after expiry', async () => {
        const fixture = await render(warningOf(LicenseSeverity.Expired, 0));

        expect(bannerOf(fixture)?.classList.contains('expired')).toBe(true);
        expect(
            fixture.nativeElement.querySelector('[data-testid="license-banner-dismiss"]')
        ).toBeNull();
    });

    it('hides itself when dismissed', async () => {
        const fixture = await render(warningOf(LicenseSeverity.Warning, 14));

        fixture.nativeElement
            .querySelector('[data-testid="license-banner-dismiss"]')
            .dispatchEvent(new MouseEvent('click'));
        fixture.detectChanges();

        expect(bannerOf(fixture)).toBeNull();
    });

    it('stays hidden for the rest of the day it was dismissed on', async () => {
        localStorage.setItem(DISMISSED_KEY, format(new Date(), 'yyyy-MM-dd'));

        const fixture = await render(warningOf(LicenseSeverity.Warning, 14));

        expect(bannerOf(fixture)).toBeNull();
    });

    it('comes back the next day', async () => {
        localStorage.setItem(DISMISSED_KEY, format(subDays(new Date(), 1), 'yyyy-MM-dd'));

        const fixture = await render(warningOf(LicenseSeverity.Warning, 14));

        expect(bannerOf(fixture)).not.toBeNull();
    });

    it('ignores a stale dismissal once expiry is close', async () => {
        localStorage.setItem(DISMISSED_KEY, format(new Date(), 'yyyy-MM-dd'));

        const fixture = await render(warningOf(LicenseSeverity.Error, 6));

        expect(bannerOf(fixture)).not.toBeNull();
    });
});
