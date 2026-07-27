import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    forwardRef,
    Input,
    OnChanges,
    SimpleChanges,
    inject
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-state-dropdown',
    templateUrl: './state-dropdown.component.html',
    styleUrls: ['./state-dropdown.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateDropdownComponent),
            multi: true
        }
    ]
})
export class StateDropdownComponent implements ControlValueAccessor, OnChanges {
    @Input() public multi = false;
    @Input() public inputId: string;
    @Input() public states: IssueState[] = [];
    /** Placeholder shown when nothing is selected (e.g. "No mapping"). */
    @Input() public placeholder?: string;
    /** Drives the floating auto-save chip on the trigger (forwarded to ui-select). */
    @Input() public saveStatus: UiSaveState = UiSaveState.Idle;

    public value: number | null = null;
    public multiValue: IssueState[] = [];

    private pendingMultiIds: number[] = [];

    public onChange: any = () => {};
    public onTouch: any = () => {};

    private readonly cdr = inject(ChangeDetectorRef);

    public ngOnChanges(changes: SimpleChanges): void {
        if (changes['states'] && this.multi) {
            this.multiValue = (this.states ?? []).filter(s =>
                this.pendingMultiIds.includes(s.idState)
            );
            this.cdr.markForCheck();
        }
    }

    public onValueChange(value: number | null): void {
        this.value = value;
        this.onChange(value);
        this.onTouch(value);
    }

    public onMultiValueChange(states: IssueState[]): void {
        this.multiValue = states;
        const ids = states.map(s => s.idState);
        this.onChange(ids);
        this.onTouch(ids);
    }

    public writeValue(value: any): void {
        if (this.multi) {
            const ids: number[] = value ?? [];
            this.pendingMultiIds = ids;
            this.multiValue = (this.states ?? []).filter(s => ids.includes(s.idState));
        } else {
            this.value = value;
        }
        this.cdr.markForCheck();
    }

    public registerOnChange(fn: any): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: any): void {
        this.onTouch = fn;
    }
}
