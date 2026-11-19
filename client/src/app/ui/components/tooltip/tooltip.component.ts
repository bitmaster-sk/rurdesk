import {
    ChangeDetectionStrategy,
    Component,
    TemplateRef,
    ViewEncapsulation,
    computed,
    input
} from '@angular/core';

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
        <div class="ui-tooltip" role="tooltip" [id]="tooltipId()" [class.is-rich]="template()">
            @if (template(); as tpl) {
                <ng-container [ngTemplateOutlet]="tpl" />
            } @else {
                {{ text() }}
            }
        </div>
    `
})
export class UiTooltipComponent {
    public readonly text = input<string | TemplateRef<unknown>>('');
    public readonly tooltipId = input<string>('');

    protected readonly template = computed(() => {
        const text = this.text();
        return text instanceof TemplateRef ? text : null;
    });
}
