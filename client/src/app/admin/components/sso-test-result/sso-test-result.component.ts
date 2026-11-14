import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { SsoTestResultConverter } from '../../converter/sso-test-result.converter';
import { SsoTestResult } from '../../model/admin-sso.model';

@Component({
    selector: 'app-sso-test-result',
    templateUrl: './sso-test-result.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SsoTestResultComponent {
    public readonly result = input<SsoTestResult | null>(null);

    protected readonly errorKey = computed(() =>
        SsoTestResultConverter.toErrorKey(this.result()?.error)
    );
    protected readonly outcomeKey = computed(() =>
        SsoTestResultConverter.toOutcomeKey(this.result()?.action)
    );
    protected readonly reasonKey = computed(() => {
        const result = this.result();
        return result ? SsoTestResultConverter.toReasonKey(result) : null;
    });
    protected readonly warningKeys = computed(() =>
        SsoTestResultConverter.toWarningKeys(this.result()?.warnings ?? [])
    );
    protected readonly groupsText = computed(() => this.result()?.groups.join(', ') ?? '');
}
