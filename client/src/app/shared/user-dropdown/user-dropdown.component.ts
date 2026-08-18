import { Component, forwardRef, Input } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { User } from 'src/app/auth/model/user.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

/** `optionValue="idUser"`, so the bound value is an id (multi: a list of ids). */
type UserDropdownValue = number | number[] | null;

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
    @Input() public multi = false;
    @Input() public filter = false;
    @Input() public appendTo: string | null = null;

    @Input() public users: User[] = [];
    /** Drives the floating auto-save chip on the trigger (forwarded to ui-select). */
    @Input() public saveStatus: UiSaveState = UiSaveState.Idle;

    public value: UserDropdownValue = null;

    public set selected(value: UserDropdownValue) {
        this.value = value;
        this.onChange(value);
        this.onTouch(value);
    }

    public get selected(): UserDropdownValue {
        return this.value;
    }

    public onChange: (value: UserDropdownValue) => void = () => {};
    public onTouch: (value: UserDropdownValue) => void = () => {};

    public writeValue(value: UserDropdownValue): void {
        this.value = value;
    }

    public registerOnChange(fn: (value: UserDropdownValue) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: (value: UserDropdownValue) => void): void {
        this.onTouch = fn;
    }
}
