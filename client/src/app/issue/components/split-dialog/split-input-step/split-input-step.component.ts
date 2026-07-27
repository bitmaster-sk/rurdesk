import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { FormControl } from '@angular/forms';

@Component({
    selector: 'app-split-input-step',
    templateUrl: './split-input-step.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class SplitInputStepComponent {
    public isLoading = input.required<boolean>();
    public issueTitle = input.required<string>();

    public split = output<string>();
    public cancel = output<void>();

    public readonly hintControl = new FormControl('');

    public onSplit(): void {
        this.split.emit(this.hintControl.value ?? '');
    }

    public onCancel(): void {
        this.cancel.emit();
    }
}
