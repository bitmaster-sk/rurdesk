import { AbstractControl, ValidationErrors } from '@angular/forms';
import { DurationConverter } from './duration.converter';
import { DurationParser } from './duration.parser';

export class DurationValidator {
    public static duration(c: AbstractControl<unknown>): ValidationErrors | null {
        const value = c.value;
        const text =
            typeof value === 'string' ? value : typeof value === 'number' ? String(value) : null;
        const valid =
            !value ||
            (text !== null &&
                DurationConverter.durationToSeconds(DurationParser.stringToDuration(text)) > 0);
        return valid
            ? null
            : {
                  validateDuration: {
                      valid: false
                  }
              };
    }
}
