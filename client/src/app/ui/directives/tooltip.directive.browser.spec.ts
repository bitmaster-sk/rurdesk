import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <button #b [uiTooltip]="text" uiTooltipPosition="top">host</button>
    `
})
class HostComponent {
    public text = 'Tip text';
}

const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
// Directive delays show by 150ms; give it margin.
const AFTER_DELAY = 220;

describe('UiTooltipDirective (browser)', () => {
    function bubble(): HTMLElement | null {
        return document.querySelector('.ui-tooltip');
    }
    function host(el: HTMLElement): HTMLElement {
        return el.querySelector('button') as HTMLElement;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    function setup(configure?: (host: HostComponent) => void) {
        const fixture = TestBed.createComponent(HostComponent);
        configure?.(fixture.componentInstance);
        fixture.detectChanges();
        return fixture;
    }

    it('shows the bubble with the text after hovering', async () => {
        const fixture = setup();
        host(fixture.nativeElement).dispatchEvent(new MouseEvent('mouseenter'));
        await wait(AFTER_DELAY);
        expect(bubble()).not.toBeNull();
        expect(bubble()!.textContent).toContain('Tip text');
        expect(bubble()!.getAttribute('role')).toBe('tooltip');
    });

    it('hides on mouseleave', async () => {
        const fixture = setup();
        const el = host(fixture.nativeElement);
        el.dispatchEvent(new MouseEvent('mouseenter'));
        await wait(AFTER_DELAY);
        el.dispatchEvent(new MouseEvent('mouseleave'));
        expect(bubble()).toBeNull();
    });

    it('cancels a pending show when the pointer leaves before the delay', async () => {
        const fixture = setup();
        const el = host(fixture.nativeElement);
        el.dispatchEvent(new MouseEvent('mouseenter'));
        el.dispatchEvent(new MouseEvent('mouseleave'));
        await wait(AFTER_DELAY);
        expect(bubble()).toBeNull();
    });

    it('shows on focus and hides on blur', async () => {
        const fixture = setup();
        const el = host(fixture.nativeElement);
        el.dispatchEvent(new FocusEvent('focus'));
        await wait(AFTER_DELAY);
        expect(bubble()).not.toBeNull();
        el.dispatchEvent(new FocusEvent('blur'));
        expect(bubble()).toBeNull();
    });

    it('does nothing when the text is empty', async () => {
        const fixture = setup(host => (host.text = ''));
        host(fixture.nativeElement).dispatchEvent(new MouseEvent('mouseenter'));
        await wait(AFTER_DELAY);
        expect(bubble()).toBeNull();
    });

    it('sets aria-describedby while shown and removes it on hide', async () => {
        const fixture = setup();
        const el = host(fixture.nativeElement);
        el.dispatchEvent(new MouseEvent('mouseenter'));
        await wait(AFTER_DELAY);
        expect(el.getAttribute('aria-describedby')).toBeTruthy();
        el.dispatchEvent(new MouseEvent('mouseleave'));
        expect(el.hasAttribute('aria-describedby')).toBe(false);
    });
});
