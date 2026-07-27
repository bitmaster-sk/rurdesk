import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    forwardRef,
    input,
    signal
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-severity-dropdown',
    templateUrl: './severity-dropdown.component.html',
    styleUrls: ['./severity-dropdown.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => SeverityDropdownComponent),
            multi: true
        }
    ],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SeverityDropdownComponent implements ControlValueAccessor {
    public readonly multi = input(false);

    public readonly inputId = input<string>();

    /** Drives the floating auto-save chip on the trigger (forwarded to ui-select). */
    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);

    // Async bindings (`severities$ | async`) emit null before the first value;
    // coalesce to [] so the selection logic never touches null.
    public readonly severities = input<IssueSeverity[] | null>([]);
    protected readonly options = computed<IssueSeverity[]>(() => this.severities() ?? []);

    protected readonly value = signal<number | null>(null);
    protected readonly multiValue = signal<IssueSeverity[]>([]);

    private readonly pendingMultiIds = signal<number[]>([]);

    private onChange: (value: unknown) => void = () => {};
    private onTouch: (value: unknown) => void = () => {};

    constructor() {
        // Resolve pending ids against the option list whenever either changes.
        effect(() => {
            if (!this.multi()) {
                return;
            }
            const ids = this.pendingMultiIds();
            this.multiValue.set(this.options().filter(s => ids.includes(s.idSeverity)));
        });
    }

    protected onValueChange(value: number | null): void {
        this.value.set(value);
        this.onChange(value);
        this.onTouch(value);
    }

    protected onMultiValueChange(severities: IssueSeverity[]): void {
        this.multiValue.set(severities);
        const ids = severities.map(s => s.idSeverity);
        this.onChange(ids);
        this.onTouch(ids);
    }

    public writeValue(value: unknown): void {
        if (this.multi()) {
            this.pendingMultiIds.set((value as number[]) ?? []);
        } else {
            this.value.set((value as number | null) ?? null);
        }
    }

    public registerOnChange(fn: (value: unknown) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: (value: unknown) => void): void {
        this.onTouch = fn;
    }
}
