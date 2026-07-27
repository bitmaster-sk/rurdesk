import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <input type="checkbox" uiToggle [(ngModel)]="on" />
    `
})
class HostComponent {
    public on = false;
}

describe('UiToggleDirective (browser)', () => {
    function input(el: HTMLElement): HTMLInputElement {
        return el.querySelector('input') as HTMLInputElement;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule, FormsModule]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('carries the ui-toggle class and switch role', () => {
        const fixture = setup();
        const el = input(fixture.nativeElement);
        expect(el.classList).toContain('ui-toggle');
        expect(el.getAttribute('role')).toBe('switch');
    });

    it('clicking updates the ngModel-bound value', async () => {
        const fixture = setup();
        input(fixture.nativeElement).click();
        fixture.detectChanges();
        await fixture.whenStable();
        expect(fixture.componentInstance.on).toBe(true);
    });

    it('reflects a programmatic value onto the DOM checked state', async () => {
        // Set the model before the first change detection to avoid the two-way
        // ngModel re-check that a mid-flight mutation would trigger in the test.
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.on = true;
        fixture.detectChanges();
        await fixture.whenStable();
        expect(input(fixture.nativeElement).checked).toBe(true);
    });
});
