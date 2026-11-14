import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { SsoState } from '../../model/admin-sso.model';

@Component({
    selector: 'app-sso-enforcement',
    templateUrl: './sso-enforcement.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoEnforcementComponent {
    public readonly state = input.required<SsoState>();
    public readonly isBusy = input(false);
    public readonly enforcedChange = output<boolean>();

    protected readonly hasBreakGlass = computed(() => this.state().breakGlassAccounts.length > 0);
    protected readonly canEnforce = computed(() => {
        const state = this.state();
        return state.isLicensed && state.isEnabled && this.hasBreakGlass() && !this.isBusy();
    });
}
