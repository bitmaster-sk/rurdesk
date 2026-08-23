import { ChangeDetectionStrategy, Component, forwardRef, input, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';

@Component({
    selector: 'app-issue-type-badge-selector',
    templateUrl: './issue-type-badge-selector.component.html',
    styleUrls: ['../badge-selector/badge-selector.shared.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => IssueTypeBadgeSelectorComponent),
            multi: true
        }
    ]
})
export class IssueTypeBadgeSelectorComponent implements ControlValueAccessor {
    public readonly issueTypes = input<IssueType[]>([]);

    protected readonly value = signal<number | null>(null);

    private onChange: (v: number | null) => void = () => {};
    private onTouched: () => void = () => {};

    protected onSelect(issueType: IssueType): void {
        this.value.set(issueType.idIssueType);
        this.onChange(this.value());
        this.onTouched();
    }

    public writeValue(value: number | null): void {
        this.value.set(value ?? null);
    }

    public registerOnChange(fn: (v: number | null) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }
}
