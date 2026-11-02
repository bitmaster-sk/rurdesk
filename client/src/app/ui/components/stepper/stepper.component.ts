import { ChangeDetectionStrategy, Component, ViewEncapsulation, input, model } from '@angular/core';

export interface UiStepperStep {
    label: string;
    isDisabled?: boolean;
}

@Component({
    selector: 'ui-stepper',
    standalone: false,
    templateUrl: './stepper.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: { class: 'ui-stepper' }
})
export class UiStepperComponent {
    public readonly steps = input.required<UiStepperStep[]>();
    public readonly active = model(0);

    protected onSelect(index: number): void {
        this.active.set(index);
    }
}
