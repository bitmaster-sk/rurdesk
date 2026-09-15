import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { format, parseISO } from 'date-fns';
import { LicenseStore } from 'src/app/core/license/license.store';
import { LicenseSeverity } from 'src/app/core/license/model/license.model';

const DISMISSED_KEY = 'rurdesk.license-banner-dismissed-on';
const RENEW_URL = 'https://rurdesk.com/docs/licensing.html';

@Component({
    selector: 'app-license-banner',
    templateUrl: './license-banner.component.html',
    styleUrls: ['./license-banner.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class LicenseBannerComponent {
    private readonly licenses = inject(LicenseStore);

    private readonly dismissedOn = signal<string | null>(localStorage.getItem(DISMISSED_KEY));

    protected readonly severities = LicenseSeverity;

    protected readonly renewUrl = RENEW_URL;

    protected readonly license = computed(() => this.licenses.state());

    protected readonly isVisible = computed(() => {
        const license = this.license();
        if (license.severity === LicenseSeverity.None) {
            return false;
        }
        if (!license.isDismissible) {
            return true;
        }
        return this.dismissedOn() !== this.today();
    });

    protected readonly expiresAtLabel = computed(() => {
        const expiresAt = this.license().expiresAt;
        return expiresAt ? format(parseISO(expiresAt), 'd MMM yyyy') : '';
    });

    protected onDismiss(): void {
        const today = this.today();
        localStorage.setItem(DISMISSED_KEY, today);
        this.dismissedOn.set(today);
    }

    private today(): string {
        return format(new Date(), 'yyyy-MM-dd');
    }
}
