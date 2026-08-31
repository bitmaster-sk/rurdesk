import { FormControl } from '@angular/forms';
import { describe, expect, it } from 'vitest';
import { NotBlankValidator } from './not-blank.validator';

function control(value: string): FormControl<string> {
    return new FormControl(value, {
        nonNullable: true,
        validators: [NotBlankValidator.validate]
    });
}

describe('NotBlankValidator', () => {
    it('accepts a value with visible characters', () => {
        expect(control('CI laptop').valid).toBe(true);
    });

    it('rejects an empty value', () => {
        expect(control('').valid).toBe(false);
    });

    it('rejects a value made only of whitespace', () => {
        expect(control('   \t ').valid).toBe(false);
    });

    it('reports the failure as a required error', () => {
        expect(control(' ').errors).toEqual({ required: true });
    });
});
