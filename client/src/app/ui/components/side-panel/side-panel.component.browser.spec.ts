import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { userEvent } from 'vitest/browser';
import { afterEach, describe, expect, it } from 'vitest';
import { TablerIconComponent } from '@tabler/icons-angular';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';
// The tokens and the panel's global styles; the geometry under test is expressed in CSS.
import 'src/styles.scss';

const KEY = 'sidePanelHidden';

@Component({
    standalone: false,
    template: `
        <div style="display: flex; height: 600px">
            <ui-side-panel title="Agent chat" hideLabel="Hide panel" [isLoading]="isLoading()">
                <button uiPanelActions type="button" class="ui-side-panel-button">+</button>
                <label uiPanelFilter class="ui-side-panel-filter"><input /></label>
                <ui-side-panel-section title="Pinned">
                    <button
                        uiSectionAction
                        type="button"
                        class="ui-side-panel-button ui-side-panel-button--small"
                        data-testid="section-add"
                    >
                        <tabler-icon icon="plus" [size]="16" />
                    </button>
                    @for (row of rows(); track row.id) {
                        <ui-side-panel-item>
                            <a
                                uiSidePanelRow
                                href="#"
                                icon="file-text"
                                [label]="row.label"
                                [isActive]="row.id === idActive()"
                            >
                                <span uiRowTrail class="ui-side-panel-meta">3 Nov</span>
                            </a>
                            <button
                                uiRowActions
                                type="button"
                                class="ui-side-panel-button"
                                data-testid="row-actions"
                            >
                                ⋮
                            </button>
                        </ui-side-panel-item>
                    }
                </ui-side-panel-section>
                <a uiPanelFooter uiSidePanelRow href="#" icon="trash" label="Trash"></a>
            </ui-side-panel>
            <main><ui-side-panel-show label="Show panel" /></main>
        </div>
    `
})
class HostComponent {
    public readonly rows = signal([
        { id: 1, label: 'Short' },
        { id: 2, label: 'Second' }
    ]);
    public readonly idActive = signal(2);
    public readonly isLoading = signal(false);
}

function setup(): { element: HTMLElement; detect: () => void; host: HostComponent } {
    TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [UiModule, TablerIconComponent, TranslateModule.forRoot()]
    });
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    return {
        element: fixture.nativeElement as HTMLElement,
        detect: () => fixture.detectChanges(),
        host: fixture.componentInstance
    };
}

function rem(): number {
    return parseFloat(getComputedStyle(document.documentElement).fontSize);
}

describe('UiSidePanelComponent', () => {
    afterEach(() => localStorage.removeItem(KEY));

    it('hides from its header button and shows again from ui-side-panel-show', () => {
        const { element, detect } = setup();
        const panel = element.querySelector<HTMLElement>('ui-side-panel')!;
        expect(element.querySelector('[data-testid="side-panel-show"]')).toBeNull();

        element.querySelector<HTMLElement>('[data-testid="side-panel-hide"]')!.click();
        detect();

        expect(panel.getBoundingClientRect().width).toBe(0);
        const show = element.querySelector<HTMLElement>('[data-testid="side-panel-show"]');
        expect(show?.getAttribute('aria-label')).toBe('Show panel');

        show!.click();
        detect();

        expect(panel.getBoundingClientRect().width).toBeGreaterThan(0);
        expect(element.querySelector('[data-testid="side-panel-show"]')).toBeNull();
    });

    it('keeps its width with a long label', () => {
        const { element, detect, host } = setup();
        host.rows.set([{ id: 1, label: 'x'.repeat(200) }]);
        detect();

        const panel = element.querySelector<HTMLElement>('ui-side-panel')!;
        expect(panel.getBoundingClientRect().width).toBe(16 * rem());
    });

    it('clamps a long label to two lines and names it in full on hover', () => {
        const { element, detect, host } = setup();
        const label = 'A conversation title that goes on '.repeat(6).trim();
        host.rows.set([{ id: 1, label }]);
        detect();

        const row = element.querySelector<HTMLElement>('[uiSidePanelRow]')!;
        const lineHeight = parseFloat(
            getComputedStyle(row.querySelector('.ui-side-panel-row-label')!).lineHeight
        );
        const padding = parseFloat(getComputedStyle(row).paddingTop) * 2;
        expect(row.getBoundingClientRect().height).toBeLessThanOrEqual(
            2 * lineHeight + padding + 1
        );
        expect(row.title).toBe(label);
    });

    it('puts its icons on whole pixels so their thin strokes stay sharp', () => {
        const { element } = setup();

        const icons = Array.from(element.querySelectorAll<SVGElement>('ui-side-panel svg'));
        expect(icons.length).toBeGreaterThan(2);
        for (const icon of icons) {
            const box = icon.getBoundingClientRect();
            expect([box.x % 1, box.y % 1], icon.outerHTML).toEqual([0, 0]);
        }
    });

    it('lines the header, section and row trail icons up on one right edge', () => {
        const { element } = setup();
        const right = (selector: string): number =>
            element.querySelector(selector)!.getBoundingClientRect().right;

        const trail = right('.ui-side-panel-row-trail');
        expect(right('[data-testid="side-panel-hide"] svg')).toBe(trail);
        expect(right('[data-testid="section-add"] svg')).toBe(trail);
    });

    it('keeps the section button, hovered, clear of the first row', () => {
        const { element } = setup();
        const add = element.querySelector('[data-testid="section-add"]')!.getBoundingClientRect();
        const head = element.querySelector('.ui-side-panel-section-head')!.getBoundingClientRect();
        const row = element.querySelector('[uiSidePanelRow]')!.getBoundingClientRect();

        expect(add.top).toBeGreaterThanOrEqual(head.top);
        expect(add.bottom).toBeLessThan(row.top);
    });

    it('shows a loader in the middle of its body instead of the rows while loading', () => {
        const { element, detect, host } = setup();
        expect(element.querySelector('[data-testid="side-panel-loading"]')).toBeNull();

        host.isLoading.set(true);
        detect();

        const body = element.querySelector('.ui-side-panel-body')!.getBoundingClientRect();
        const icon = element
            .querySelector('[data-testid="side-panel-loading"] svg')!
            .getBoundingClientRect();
        expect(element.querySelector('[uiSidePanelRow]:not([uiPanelFooter])')).toBeNull();
        expect(icon.left + icon.width / 2).toBeCloseTo(body.left + body.width / 2, 0);
        expect(icon.top + icon.height / 2).toBeCloseTo(body.top + body.height / 2, 0);
        expect(element.querySelector('[uiPanelFooter]')).not.toBeNull();
    });

    it('marks the active row as the current page', () => {
        const { element } = setup();

        const current = element.querySelectorAll('[aria-current="page"]');
        expect(current).toHaveLength(1);
        expect(current[0].getAttribute('title')).toBe('Second');
    });

    it('shows row actions instead of the trail on hover and keyboard focus', async () => {
        const { element } = setup();
        const item = element.querySelector<HTMLElement>('ui-side-panel-item')!;
        const actions = item.querySelector<HTMLElement>('[data-testid="row-actions"]')!;
        const trail = item.querySelector<HTMLElement>('.ui-side-panel-row-trail')!;
        expect(getComputedStyle(trail).visibility).toBe('visible');
        expect(getComputedStyle(actions.parentElement!).opacity).toBe('0');

        item.querySelector<HTMLElement>('[uiSidePanelRow]')!.focus();
        await userEvent.tab();

        expect(document.activeElement).toBe(actions);
        expect(getComputedStyle(trail).visibility).toBe('hidden');
        expect(getComputedStyle(actions.parentElement!).opacity).toBe('1');
    });
});
