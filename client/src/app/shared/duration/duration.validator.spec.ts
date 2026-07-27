import type { FormControl } from '@angular/forms';
import { DurationValidator } from './duration.validator';

function control(value: string): FormControl {
    return { value } as unknown as FormControl;
}

describe('DurationValidator.duration', () => {
    it('accepts an empty value', () => {
        expect(DurationValidator.duration(control(''))).toBeNull();
    });

    it('accepts a positive duration', () => {
        expect(DurationValidator.duration(control('2h'))).toBeNull();
        expect(DurationValidator.duration(control('30m'))).toBeNull();
    });

    it('rejects a zero duration', () => {
        expect(DurationValidator.duration(control('0'))).toEqual({
            validateDuration: { valid: false }
        });
    });

    it('rejects an unparseable value', () => {
        expect(DurationValidator.duration(control('abc'))).toEqual({
            validateDuration: { valid: false }
        });
    });
});
