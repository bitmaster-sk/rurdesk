import { ChangeDetectionStrategy, Component, ViewEncapsulation, inject } from '@angular/core';
import { UiToastSeverity, UiToastService } from '../../service/ui-toast.service';

/**
 * Fixed top-right stack of product toasts. Reads the visible list from
 * {@link UiToastService} and renders each with a severity icon, message, close
 * button and a CSS-driven progress bar. Mounted once in the app root. Not a CDK
 * overlay — a plain `position:fixed` container at z-index 12000 (above
 * `.cdk-overlay-container`).
 */
@Component({
    selector: 'ui-toast',
    standalone: false,
    templateUrl: './toast.component.html',
    // Styles are global (src/app/ui/ui.styles.scss); no styleUrl → no FOUC.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None
})
export class UiToastComponent {
    protected readonly toast = inject(UiToastService);

    private readonly icons: Record<UiToastSeverity, string> = {
        success: 'circle-check',
        info: 'info-circle',
        error: 'alert-circle'
    };

    protected icon(severity: UiToastSeverity): string {
        return this.icons[severity];
    }
}
