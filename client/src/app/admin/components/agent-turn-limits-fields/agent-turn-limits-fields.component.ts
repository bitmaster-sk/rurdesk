import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { ControlContainer, FormGroupDirective } from '@angular/forms';

@Component({
    selector: 'app-agent-turn-limits-fields',
    templateUrl: './agent-turn-limits-fields.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    viewProviders: [{ provide: ControlContainer, useExisting: FormGroupDirective }]
})
export class AgentTurnLimitsFieldsComponent {
    public readonly idPrefix = input.required<string>();
}
