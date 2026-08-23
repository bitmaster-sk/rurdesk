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
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-issue-type-dropdown',
    templateUrl: './issue-type-dropdown.component.html',
    styleUrls: ['./issue-type-dropdown.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => IssueTypeDropdownComponent),
            multi: true
        }
    ],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssueTypeDropdownComponent implements ControlValueAccessor {
    public readonly multi = input(false);

    public readonly inputId = input<string>();

    public readonly placeholder = input<string>();

    public readonly saveStatus = input<UiSaveState>(UiSaveState.Idle);

    public readonly issueTypes = input<IssueType[] | null>([]);
    protected readonly options = computed<IssueType[]>(() => this.issueTypes() ?? []);

    protected readonly value = signal<number | null>(null);
    protected readonly multiValue = signal<IssueType[]>([]);

    private readonly pendingMultiIds = signal<number[]>([]);

    private onChange: (value: unknown) => void = () => {};
    private onTouch: (value: unknown) => void = () => {};

    public constructor() {
        effect(() => {
            if (!this.multi()) {
                return;
            }
            const ids = this.pendingMultiIds();
            this.multiValue.set(this.options().filter(t => ids.includes(t.idIssueType)));
        });
    }

    protected onValueChange(value: number | null): void {
        this.value.set(value);
        this.onChange(value);
        this.onTouch(value);
    }

    protected onMultiValueChange(issueTypes: IssueType[]): void {
        this.multiValue.set(issueTypes);
        const ids = issueTypes.map(t => t.idIssueType);
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
