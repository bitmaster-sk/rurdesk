import { describe, it, expect, beforeEach } from 'vitest';
import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <div uiActivatable (click)="activations = activations + 1">
            <button type="button" (click)="$event.stopPropagation(); inner = inner + 1">
                Inner
            </button>
        </div>
    `
})
class HostComponent {
    public activations = 0;
    public inner = 0;
}

describe('UiActivatableDirective (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return {
            fixture,
            host: fixture.nativeElement.querySelector('[uiActivatable]') as HTMLElement,
            inner: fixture.nativeElement.querySelector('button') as HTMLElement
        };
    }

    it('puts the element in the tab order and exposes it as a button', () => {
        const { host } = setup();
        expect(host.getAttribute('tabindex')).toBe('0');
        expect(host.getAttribute('role')).toBe('button');
    });

    it('activates on Enter and on Space', () => {
        const { fixture, host } = setup();

        host.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        host.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
        fixture.detectChanges();

        expect(fixture.componentInstance.activations).toBe(2);
    });

    it('ignores other keys', () => {
        const { fixture, host } = setup();
        host.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true }));
        fixture.detectChanges();
        expect(fixture.componentInstance.activations).toBe(0);
    });

    it('leaves a nested control its own keys', () => {
        const { fixture, inner } = setup();

        inner.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
        fixture.detectChanges();

        expect(fixture.componentInstance.activations).toBe(0);
    });
});
