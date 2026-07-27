import { Component, forwardRef, Input } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { User } from 'src/app/auth/model/user.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-user-dropdown',
    templateUrl: './user-dropdown.component.html',
    styleUrls: ['./user-dropdown.component.scss'],
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UserDropdownComponent),
            multi: true
        }
    ],
    standalone: false
})
export class UserDropdownComponent implements ControlValueAccessor {
    @Input() multi = false;
    @Input() filter = false;
    @Input() appendTo: any = null;

    @Input() users: User[] = [];
    /** Drives the floating auto-save chip on the trigger (forwarded to ui-select). */
    @Input() saveStatus: UiSaveState = UiSaveState.Idle;

    value: any;

    public set selected(value) {
        this.value = value;
        this.onChange(value);
        this.onTouch(value);
    }

    public get selected(): any {
        return this.value;
    }

    constructor() {}

    public onChange: any = () => {};
    public onTouch: any = () => {};

    public writeValue(value: any): void {
        this.value = value;
    }

    public registerOnChange(fn: any): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: any): void {
        this.onTouch = fn;
    }
}
