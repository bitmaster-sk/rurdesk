import { FormControl, FormGroup } from '@angular/forms';
import { describe, expect, it } from 'vitest';
import { DATE_ORDER_ERROR, dateOrder } from './date-order.validator';

function group(from: Date | null, to: Date | null): FormGroup {
    return new FormGroup(
        {
            from: new FormControl<Date | null>(from),
            to: new FormControl<Date | null>(to),
            other: new FormControl<Date | null>(null)
        },
        { validators: [dateOrder('from', 'to')] }
    );
}

describe('dateOrder', () => {
    it('accepts a range that moves forward', () => {
        expect(group(new Date(2026, 7, 1), new Date(2026, 7, 15)).valid).toBe(true);
    });

    it('rejects a range that ends before it starts', () => {
        expect(group(new Date(2026, 7, 20), new Date(2026, 7, 10)).valid).toBe(false);
    });

    it('rejects a range that ends on the day it starts', () => {
        expect(group(new Date(2026, 7, 20, 9, 0), new Date(2026, 7, 20, 18, 0)).valid).toBe(false);
    });

    it('stays quiet until both ends are filled in', () => {
        expect(group(new Date(2026, 7, 20), null).valid).toBe(true);
        expect(group(null, new Date(2026, 7, 20)).valid).toBe(true);
        expect(group(null, null).valid).toBe(true);
    });

    it('reports which pair of controls it was watching', () => {
        const form = group(new Date(2026, 7, 20), new Date(2026, 7, 10));
        expect(form.errors?.[DATE_ORDER_ERROR]).toEqual({ startKey: 'from', endKey: 'to' });
    });

    it('compares the picked days, not the instants, so a timezone cannot flip it', () => {
        const form = new FormGroup(
            {
                a: new FormControl<Date | null>(new Date(2026, 7, 10, 23, 59)),
                b: new FormControl<Date | null>(new Date(2026, 7, 11, 0, 1))
            },
            { validators: [dateOrder('a', 'b')] }
        );
        expect(form.valid).toBe(true);
    });

    it('ignores controls it was not pointed at', () => {
        const form = group(new Date(2026, 7, 1), new Date(2026, 7, 15));
        form.get('other')!.setValue(new Date(1990, 0, 1));
        form.updateValueAndValidity();
        expect(form.valid).toBe(true);
    });
});
