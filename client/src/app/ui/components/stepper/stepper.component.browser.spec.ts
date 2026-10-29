import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { IconCheck, TablerIconComponent, provideTablerIcons } from '@tabler/icons-angular';
import { describe, expect, it } from 'vitest';
import { UiStepperComponent, UiStepperStep } from './stepper.component';

@Component({
    template: '<ui-stepper [steps]="steps" [(active)]="active" />',
    standalone: false
})
class HostComponent {
    public readonly steps: UiStepperStep[] = [
        { label: 'Tag' },
        { label: 'Tasks' },
        { label: 'Notes', isDisabled: true }
    ];
    public readonly active = signal(1);
}

function render() {
    TestBed.configureTestingModule({
        declarations: [HostComponent, UiStepperComponent],
        imports: [TablerIconComponent],
        providers: [provideTablerIcons({ IconCheck })]
    });
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    const element: HTMLElement = fixture.nativeElement;
    const items = () => Array.from(element.querySelectorAll<HTMLElement>('.ui-stepper__item'));
    const buttons = () => items().map(item => item.querySelector('button')!);
    return { fixture, items, buttons };
}

describe('UiStepperComponent', () => {
    it('marks the steps before the active one as done and the active one as current', () => {
        const { items } = render();

        expect(items()[0].classList).toContain('ui-stepper__item--done');
        expect(items()[0].querySelector('tabler-icon')).not.toBeNull();
        expect(items()[1].classList).toContain('ui-stepper__item--current');
        expect(items()[1].querySelector('button')!.getAttribute('aria-current')).toBe('step');
        expect(items()[2].classList).not.toContain('ui-stepper__item--done');
        expect(items()[2].textContent).toContain('3');
    });

    it('moves to a step that is clicked and ignores disabled ones', () => {
        const { fixture, buttons } = render();

        expect(buttons()[2].disabled).toBe(true);
        buttons()[0].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.active()).toBe(0);
        expect(buttons()[0].getAttribute('aria-current')).toBe('step');
    });
});
