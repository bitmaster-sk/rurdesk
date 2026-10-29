import { ChangeDetectionStrategy, Component, forwardRef, input, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

@Component({
    selector: 'app-tag-color-picker',
    templateUrl: './tag-color-picker.component.html',
    styleUrls: ['./tag-color-picker.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => TagColorPickerComponent),
            multi: true
        }
    ],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class TagColorPickerComponent implements ControlValueAccessor {
    public readonly colors = input.required<readonly string[]>();
    public readonly label = input<string>('');

    protected readonly value = signal<string | null>(null);
    protected readonly isDisabled = signal(false);

    private onChange: (value: string) => void = () => {};
    private onTouched: () => void = () => {};

    protected onPick(color: string): void {
        if (this.isDisabled()) {
            return;
        }
        this.value.set(color);
        this.onChange(color);
        this.onTouched();
    }

    public writeValue(value: string | null): void {
        this.value.set(value?.toLowerCase() ?? null);
    }

    public registerOnChange(fn: (value: string) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.isDisabled.set(isDisabled);
    }
}
