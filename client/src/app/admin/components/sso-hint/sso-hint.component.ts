import { ChangeDetectionStrategy, Component, input } from '@angular/core';

@Component({
    selector: 'app-sso-hint',
    templateUrl: './sso-hint.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    host: { class: 'inline-flex' }
})
export class SsoHintComponent {
    public readonly text = input.required<string>();
}
