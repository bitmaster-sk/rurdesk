import { FormControl, ValidationErrors } from '@angular/forms';
import { DurationConverter } from './duration.converter';
import { DurationParser } from './duration.parser';

export class DurationValidator {
    public static duration(c: FormControl): ValidationErrors | null {
        const valid =
            !c.value ||
            DurationConverter.durationToSeconds(DurationParser.stringToDuration(c.value)) > 0;
        return valid
            ? null
            : {
                  validateDuration: {
                      valid: false
                  }
              };
    }
}
