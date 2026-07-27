import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

/**
 * Presentational bubble rendered inside the CDK overlay by `[uiTooltip]`.
 * Not used directly in templates — attached as a `ComponentPortal`.
 */
@Component({
    selector: 'ui-tooltip',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <div class="ui-tooltip" role="tooltip" [id]="tooltipId()">{{ text() }}</div>
    `
})
export class UiTooltipComponent {
    public readonly text = input<string>('');
    public readonly tooltipId = input<string>('');
}
