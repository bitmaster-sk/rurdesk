import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { userEvent } from 'vitest/browser';
import { UiModule } from '../../ui.module';

@Component({
    standalone: false,
    template: `
        <ui-button [disabled]="disabled" (click)="onClick()">Go</ui-button>
    `
})
class HostComponent {
    public disabled = false;
    public clicks = 0;

    public onClick(): void {
        this.clicks++;
    }
}

describe('UiButtonComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    function setup(disabled: boolean) {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.disabled = disabled;
        fixture.detectChanges();
        return fixture;
    }

    function button(fixture: { nativeElement: HTMLElement }): HTMLButtonElement {
        return fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    }

    it('runs the click handler when enabled', async () => {
        const fixture = setup(false);
        await userEvent.click(button(fixture));
        expect(fixture.componentInstance.clicks).toBe(1);
    });

    it('marks the inner button disabled so it takes no input', () => {
        expect(button(setup(true)).disabled).toBe(true);
    });
});
