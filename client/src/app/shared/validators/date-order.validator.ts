import { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';
import { DateUtil } from '../date/date.util';

export abstract class DateOrderValidator {
    public static readonly ERROR = 'dateOrder';

    public static create(startKey: string, endKey: string): ValidatorFn {
        return (group: AbstractControl): ValidationErrors | null => {
            const start = group.get(startKey)?.value as Date | null;
            const end = group.get(endKey)?.value as Date | null;
            if (!start || !end) {
                return null;
            }
            return DateUtil.toUtcDay(end) > DateUtil.toUtcDay(start)
                ? null
                : { [DateOrderValidator.ERROR]: { startKey, endKey } };
        };
    }
}
