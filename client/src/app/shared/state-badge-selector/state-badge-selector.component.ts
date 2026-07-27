import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    forwardRef,
    Input,
    inject
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IssueState } from 'src/app/state/model/issue-state.model';

@Component({
    selector: 'app-state-badge-selector',
    templateUrl: './state-badge-selector.component.html',
    styleUrls: ['../badge-selector/badge-selector.shared.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateBadgeSelectorComponent),
            multi: true
        }
    ]
})
export class StateBadgeSelectorComponent implements ControlValueAccessor {
    @Input() public states: IssueState[] = [];

    public value: number | null = null;

    private onChange: (v: number | null) => void = () => {};
    private onTouched: () => void = () => {};

    private readonly cdr = inject(ChangeDetectorRef);

    public select(state: IssueState): void {
        this.value = state.idState;
        this.onChange(this.value);
        this.onTouched();
    }

    // Returns the token reference, not a literal, so this stays in lockstep with
    // the state badge and the calendar chips (all three read --ui-color-state-*).
    public stateColor(state: IssueState): string {
        if (state.start) return 'var(--ui-color-state-start)';
        if (state.final) return 'var(--ui-color-state-final)';
        return 'var(--ui-color-state-in-progress)';
    }

    public writeValue(value: number | null): void {
        this.value = value ?? null;
        this.cdr.markForCheck();
    }

    public registerOnChange(fn: (v: number | null) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }
}
