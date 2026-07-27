import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
    selector: 'app-run-recovery-banner',
    templateUrl: './run-recovery-banner.component.html',
    styleUrls: ['./run-recovery-banner.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RunRecoveryBannerComponent {
    public readonly idRun = input.required<number>();
    // Failed-stage error shown inside the banner: errorKey is an i18n key
    // (AGENT.ERROR.*) for the human-readable reason; errorDetail is the raw
    // provider/agent message. Both optional — null hides that line.
    public readonly errorKey = input<string | null>(null);
    public readonly errorDetail = input<string | null>(null);

    public readonly continued = output<void>();
    public readonly restarted = output<void>();

    protected onContinue(): void {
        this.continued.emit();
    }

    protected onRestart(): void {
        this.restarted.emit();
    }
}
