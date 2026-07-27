import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    forwardRef,
    Input,
    inject
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';

@Component({
    selector: 'app-severity-badge-selector',
    templateUrl: './severity-badge-selector.component.html',
    styleUrls: ['../badge-selector/badge-selector.shared.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => SeverityBadgeSelectorComponent),
            multi: true
        }
    ]
})
export class SeverityBadgeSelectorComponent implements ControlValueAccessor {
    @Input() public severities: IssueSeverity[] = [];

    public value: number | null = null;

    private onChange: (v: number | null) => void = () => {};
    private onTouched: () => void = () => {};

    private readonly cdr = inject(ChangeDetectorRef);

    public select(severity: IssueSeverity): void {
        this.value = severity.idSeverity;
        this.onChange(this.value);
        this.onTouched();
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
