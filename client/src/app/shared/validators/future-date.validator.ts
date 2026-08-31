import { AbstractControl, ValidationErrors } from '@angular/forms';
import { isBefore, startOfDay } from 'date-fns';

export abstract class FutureDateValidator {
    public static validate(
        this: void,
        control: AbstractControl<Date | null>
    ): ValidationErrors | null {
        const value = control.value;
        if (!value) {
            return null;
        }
        return isBefore(startOfDay(value), startOfDay(new Date())) ? { pastDate: true } : null;
    }
}
