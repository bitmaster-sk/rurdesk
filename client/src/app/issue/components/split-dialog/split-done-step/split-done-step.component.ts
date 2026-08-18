import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
    selector: 'app-split-done-step',
    templateUrl: './split-done-step.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SplitDoneStepComponent {
    public count = input.required<number>();
    public closed = output<void>();

    public onClose(): void {
        this.closed.emit();
    }
}
