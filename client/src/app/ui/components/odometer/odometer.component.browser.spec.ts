import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../../ui.module';

@Component({
    template: '<ui-odometer [value]="value()" />',
    standalone: false
})
class HostComponent {
    public readonly value = signal(0);
}

describe('UiOdometerComponent', () => {
    beforeEach(() =>
        TestBed.configureTestingModule({
            imports: [UiModule],
            declarations: [HostComponent]
        })
    );

    function digitOffsets(el: HTMLElement): string[] {
        return Array.from(el.querySelectorAll<HTMLElement>('.ui-odometer--strip')).map(
            s => s.style.transform
        );
    }

    it('renders one rolling strip per digit', () => {
        const f = TestBed.createComponent(HostComponent);
        f.componentInstance.value.set(128);
        f.detectChanges();
        expect(f.nativeElement.querySelectorAll('.ui-odometer--digit').length).toBe(3);
        expect(digitOffsets(f.nativeElement)).toEqual([
            'translateY(-10%)',
            'translateY(-20%)',
            'translateY(-80%)'
        ]);
    });

    it('rolls digits to the new value on change', () => {
        const f = TestBed.createComponent(HostComponent);
        f.componentInstance.value.set(9);
        f.detectChanges();
        f.componentInstance.value.set(12);
        f.detectChanges();
        expect(digitOffsets(f.nativeElement)).toEqual(['translateY(-10%)', 'translateY(-20%)']);
    });

    it('clamps negative values to 0 and exposes the value as aria-label', () => {
        const f = TestBed.createComponent(HostComponent);
        f.componentInstance.value.set(-5);
        f.detectChanges();
        expect(digitOffsets(f.nativeElement)).toEqual(['translateY(0%)']);
        expect(f.nativeElement.querySelector('ui-odometer')!.getAttribute('aria-label')).toBe('0');
    });
});
