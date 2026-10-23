import { AbstractControl, ValidationErrors } from '@angular/forms';
import { startOfDay } from 'date-fns';

export abstract class TrackDayValidator {
    public static notInFuture(control: AbstractControl<Date | null>): ValidationErrors | null {
        if (!control.value) {
            return null;
        }
        return startOfDay(control.value) > startOfDay(new Date()) ? { future: true } : null;
    }
}
