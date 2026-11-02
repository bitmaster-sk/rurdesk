import { ChangeDetectionStrategy, Component, input } from '@angular/core';

@Component({
    selector: 'ui-gallery-hover-card',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: '<div class="ui-popover w-60">{{ text() }}</div>'
})
export class UiGalleryHoverCardComponent {
    public readonly text = input('');
}
