import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    computed,
    input
} from '@angular/core';

export type UiMessageSeverity = 'info' | 'success' | 'warn' | 'danger';

const ICONS: Record<UiMessageSeverity, string> = {
    info: 'info-circle',
    success: 'circle-check',
    warn: 'alert-triangle',
    danger: 'alert-circle'
};

/**
 * Inline alert box. `severity` sets colour + icon via a `.ui-message--<severity>`
 * host class; body is content-projected. `role="alert"` for `danger`, else
 * `status`. Icon is decorative (`aria-hidden`) — meaning lives in the text.
 */
@Component({
    selector: 'ui-message',
    standalone: false,
    templateUrl: './message.component.html',
    // Styles are global (see src/app/ui/ui.styles.scss); no styleUrl → no FOUC.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-message',
        '[attr.role]': "severity() === 'danger' ? 'alert' : 'status'",
        '[class.ui-message--info]': "severity() === 'info'",
        '[class.ui-message--success]': "severity() === 'success'",
        '[class.ui-message--warn]': "severity() === 'warn'",
        '[class.ui-message--danger]': "severity() === 'danger'"
    }
})
export class UiMessageComponent {
    /** Colour + icon + aria-live level. Defaults to `info`. */
    public readonly severity = input<UiMessageSeverity>('info');

    protected readonly icon = computed(() => ICONS[this.severity()]);
}
