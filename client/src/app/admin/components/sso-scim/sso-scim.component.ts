import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { SsoState } from '../../model/admin-sso.model';

@Component({
    selector: 'app-sso-scim',
    templateUrl: './sso-scim.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoScimComponent {
    private readonly clipboard = inject(ClipboardService);

    public readonly state = input.required<SsoState>();
    public readonly revealedToken = input<string | null>(null);
    public readonly isBusy = input(false);
    public readonly enabledChange = output<boolean>();
    public readonly generateToken = output<void>();
    public readonly deleteToken = output<void>();

    protected readonly scim = computed(() => this.state().scim);
    protected readonly canChange = computed(() => this.state().isLicensed && !this.isBusy());

    protected onToggle(event: Event): void {
        const toggle = event.target as HTMLInputElement;
        this.enabledChange.emit(toggle.checked);
        // The saved state drives the toggle, so a failed request must not leave it flipped.
        toggle.checked = this.scim().isEnabled;
    }

    protected onCopy(value: string): void {
        void this.clipboard.copy(value);
    }
}
