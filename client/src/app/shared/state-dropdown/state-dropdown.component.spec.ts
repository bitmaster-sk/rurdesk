import { ChangeDetectorRef, Injector, runInInjectionContext } from '@angular/core';
import { describe, expect, it } from 'vitest';
import { StateDropdownComponent } from './state-dropdown.component';

describe('StateDropdownComponent.writeValue (single mode)', () => {
    function create(): StateDropdownComponent {
        const injector = Injector.create({
            providers: [{ provide: ChangeDetectorRef, useValue: { markForCheck: () => {} } }]
        });
        const component = runInInjectionContext(injector, () => new StateDropdownComponent());
        component.multi = false;
        return component;
    }

    it('accepts a number', () => {
        const c = create();
        c.writeValue(5);
        expect(c.value).toBe(5);
    });

    it('accepts null', () => {
        const c = create();
        c.writeValue(5);
        c.writeValue(null);
        expect(c.value).toBeNull();
    });

    it('coerces a numeric string to a number (regression case)', () => {
        const c = create();
        c.writeValue('5' as any);
        expect(c.value).toBe(5);
    });

    it('coerces undefined to null', () => {
        const c = create();
        c.writeValue(undefined as any);
        expect(c.value).toBeNull();
    });

    it('rejects a non-numeric string', () => {
        const c = create();
        c.writeValue('abc' as any);
        expect(c.value).toBeNull();
    });
});
