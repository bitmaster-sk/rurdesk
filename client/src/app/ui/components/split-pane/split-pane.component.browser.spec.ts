import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { UiModule } from '../../ui.module';
// The component's styles are global (ui.styles.scss) and not loaded by the test
// setup, but the collapse/min-width behaviour under test is expressed in CSS.
import './split-pane.styles.scss';

const STORAGE_KEY = 'test.split-pane';

@Component({
    standalone: false,
    template: `
        <ui-split-pane style="width: 1000px; height: 400px" [storageKey]="storageKey" [minPx]="280">
            <div uiSplitStart>start</div>
            <div uiSplitEnd>end</div>
        </ui-split-pane>
    `
})
class HostComponent {
    public readonly storageKey = STORAGE_KEY;
}

describe('UiSplitPaneComponent (browser)', () => {
    function splitter(el: HTMLElement): HTMLElement {
        return el.querySelector('[data-testid="split-pane-splitter"]')!;
    }

    function startPanel(el: HTMLElement): HTMLElement {
        return el.querySelector('.ui-split-pane__panel--start')!;
    }

    function endPanel(el: HTMLElement): HTMLElement {
        return el.querySelector('.ui-split-pane__panel--end')!;
    }

    function collapseStartBtn(el: HTMLElement): HTMLButtonElement {
        return el.querySelector('[data-testid="split-pane-collapse-start"]')!;
    }

    function collapseEndBtn(el: HTMLElement): HTMLButtonElement {
        return el.querySelector('[data-testid="split-pane-collapse-end"]')!;
    }

    function resetBtn(el: HTMLElement): HTMLButtonElement {
        return el.querySelector('[data-testid="split-pane-reset"]')!;
    }

    function drag(el: HTMLElement, clientX: number): void {
        const handle = splitter(el);
        handle.setPointerCapture = () => undefined;
        handle.dispatchEvent(
            new PointerEvent('pointerdown', { pointerId: 1, clientX: 500, bubbles: true })
        );
        handle.dispatchEvent(
            new PointerEvent('pointermove', { pointerId: 1, clientX, bubbles: true })
        );
        handle.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
    }

    beforeEach(async () => {
        localStorage.removeItem(STORAGE_KEY);
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule]
        }).compileComponents();
    });

    afterEach(() => {
        localStorage.removeItem(STORAGE_KEY);
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('splits the panes evenly before the user touches it', () => {
        const fixture = setup();
        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('50%');
    });

    it('dragging the separator widens one pane at the expense of the other', () => {
        const fixture = setup();
        const before = startPanel(fixture.nativeElement).getBoundingClientRect().width;
        const endBefore = endPanel(fixture.nativeElement).getBoundingClientRect().width;

        drag(fixture.nativeElement, 700);
        fixture.detectChanges();

        const after = startPanel(fixture.nativeElement).getBoundingClientRect().width;
        const endAfter = endPanel(fixture.nativeElement).getBoundingClientRect().width;
        expect(after).toBeGreaterThan(before);
        expect(endAfter).toBeLessThan(endBefore);
    });

    it('stops at the minimum width instead of collapsing the pane', () => {
        const fixture = setup();

        drag(fixture.nativeElement, 10);
        fixture.detectChanges();

        expect(
            startPanel(fixture.nativeElement).getBoundingClientRect().width
        ).toBeGreaterThanOrEqual(279);
    });

    it('double-clicking the separator restores the even split', () => {
        const fixture = setup();
        drag(fixture.nativeElement, 750);
        fixture.detectChanges();

        splitter(fixture.nativeElement).dispatchEvent(
            new MouseEvent('dblclick', { bubbles: true })
        );
        fixture.detectChanges();

        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('50%');
    });

    it('arrow keys move the separator, shift makes a finer step', () => {
        const fixture = setup();
        const handle = splitter(fixture.nativeElement);

        handle.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
        fixture.detectChanges();
        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('55%');

        handle.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowLeft', shiftKey: true, bubbles: true })
        );
        fixture.detectChanges();
        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('54%');
    });

    it('collapses the start pane and restores it from the same control pair', () => {
        const fixture = setup();

        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(startPanel(fixture.nativeElement).getBoundingClientRect().width).toBe(0);
        expect(collapseStartBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('true');

        collapseEndBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(startPanel(fixture.nativeElement).getBoundingClientRect().width).toBeGreaterThan(0);
    });

    it('collapses the end pane and restores it from the same control pair', () => {
        const fixture = setup();

        collapseEndBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(endPanel(fixture.nativeElement).getBoundingClientRect().width).toBe(0);
        expect(collapseEndBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('true');

        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(endPanel(fixture.nativeElement).getBoundingClientRect().width).toBeGreaterThan(0);
    });

    it('offers no reset while the split is already at its default', () => {
        const fixture = setup();
        expect(resetBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('true');

        drag(fixture.nativeElement, 700);
        fixture.detectChanges();
        expect(resetBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('false');
    });

    it('the reset control restores the even split after a drag', () => {
        const fixture = setup();
        drag(fixture.nativeElement, 750);
        fixture.detectChanges();

        resetBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('50%');
        expect(resetBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('true');
    });

    it('the reset control also brings back a collapsed pane', () => {
        const fixture = setup();
        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        resetBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        expect(startPanel(fixture.nativeElement).getBoundingClientRect().width).toBeGreaterThan(0);
        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('50%');
        expect(startPanel(fixture.nativeElement).hasAttribute('inert')).toBe(false);
    });

    it('keeps focus on the reset control after it becomes inactive', () => {
        const fixture = setup();
        drag(fixture.nativeElement, 700);
        fixture.detectChanges();

        resetBtn(fixture.nativeElement).focus();
        resetBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        expect(document.activeElement).toBe(resetBtn(fixture.nativeElement));
    });

    it('explains through its tooltip why the reset is inactive', () => {
        const fixture = setup();
        expect(resetBtn(fixture.nativeElement).getAttribute('aria-label')).toBe(
            'Already at the even split'
        );

        drag(fixture.nativeElement, 700);
        fixture.detectChanges();
        expect(resetBtn(fixture.nativeElement).getAttribute('aria-label')).toBe(
            'Reset to an even split'
        );
    });

    it('ignores a click on an inactive control', () => {
        const fixture = setup();

        resetBtn(fixture.nativeElement).click();
        collapseStartBtn(fixture.nativeElement).click();
        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        expect(startPanel(fixture.nativeElement).getBoundingClientRect().width).toBe(0);
        expect(endPanel(fixture.nativeElement).getBoundingClientRect().width).toBeGreaterThan(0);
    });

    it('forgets the stored split once it is reset', () => {
        const first = setup();
        drag(first.nativeElement, 700);
        first.detectChanges();
        resetBtn(first.nativeElement).click();
        first.detectChanges();
        first.destroy();

        const second = setup();
        expect(startPanel(second.nativeElement).style.flexBasis).toBe('50%');
    });

    it('takes a collapsed pane out of the tab order and the a11y tree', () => {
        const fixture = setup();

        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(startPanel(fixture.nativeElement).hasAttribute('inert')).toBe(true);
        expect(endPanel(fixture.nativeElement).hasAttribute('inert')).toBe(false);

        collapseEndBtn(fixture.nativeElement).click();
        collapseEndBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(endPanel(fixture.nativeElement).hasAttribute('inert')).toBe(true);
        expect(startPanel(fixture.nativeElement).hasAttribute('inert')).toBe(false);
    });

    it('keeps focus on the collapse control after it becomes inactive', () => {
        const fixture = setup();

        collapseStartBtn(fixture.nativeElement).focus();
        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();

        expect(document.activeElement).toBe(collapseStartBtn(fixture.nativeElement));
        expect(collapseEndBtn(fixture.nativeElement).getAttribute('aria-disabled')).toBe('false');
    });

    it('reports the collapsed edge through aria-valuenow', () => {
        const fixture = setup();

        collapseStartBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(splitter(fixture.nativeElement).getAttribute('aria-valuenow')).toBe('0');

        collapseEndBtn(fixture.nativeElement).click();
        collapseEndBtn(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(splitter(fixture.nativeElement).getAttribute('aria-valuenow')).toBe('100');
    });

    it('restores the stored ratio when the component is recreated', () => {
        const first = setup();
        drag(first.nativeElement, 700);
        first.detectChanges();
        const stored = startPanel(first.nativeElement).style.flexBasis;
        first.destroy();

        const second = setup();
        expect(startPanel(second.nativeElement).style.flexBasis).toBe(stored);
    });

    it('restores a collapsed pane when the component is recreated', () => {
        const first = setup();
        collapseStartBtn(first.nativeElement).click();
        first.detectChanges();
        first.destroy();

        const second = setup();
        expect(startPanel(second.nativeElement).getBoundingClientRect().width).toBe(0);
    });

    it('ignores a corrupted stored value instead of breaking the layout', () => {
        localStorage.setItem(STORAGE_KEY, 'not json');
        const fixture = setup();
        expect(startPanel(fixture.nativeElement).style.flexBasis).toBe('50%');
    });
});
