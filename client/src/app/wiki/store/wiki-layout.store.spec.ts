import { Injector } from '@angular/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SidePanelStore } from 'src/app/ui/components/side-panel/side-panel.store';
import { WikiLayoutStore } from './wiki-layout.store';

/** A fresh injector reads the saved state like a reload of the app. */
function load(): { layout: WikiLayoutStore; panel: SidePanelStore } {
    const injector = Injector.create({ providers: [WikiLayoutStore, SidePanelStore] });
    return { layout: injector.get(WikiLayoutStore), panel: injector.get(SidePanelStore) };
}

describe('WikiLayoutStore', () => {
    let saved: Map<string, string>;

    beforeEach(() => {
        saved = new Map();
        vi.stubGlobal('localStorage', {
            getItem: (key: string) => saved.get(key) ?? null,
            setItem: (key: string, value: string) => saved.set(key, value)
        });
    });

    afterEach(() => vi.unstubAllGlobals());

    it('starts narrow and remembers the full width choice for the next visit', () => {
        const { layout } = load();
        expect(layout.isFullWidth()).toBe(false);

        layout.toggleFullWidth();

        expect(layout.isFullWidth()).toBe(true);
        expect(load().layout.isFullWidth()).toBe(true);
    });

    it('hides the page tree with the shared panel and remembers that for the next visit', () => {
        const { layout, panel } = load();
        expect(layout.isHidden()).toBe(false);

        layout.hide();

        expect(layout.isHidden()).toBe(true);
        expect(panel.isHidden()).toBe(true);
        expect(load().layout.isHidden()).toBe(true);

        panel.show();
        expect(layout.isHidden()).toBe(false);
    });

    it('opens the tree as an overlay on narrow screens without touching the shared panel', () => {
        const listeners: ((event: { matches: boolean }) => void)[] = [];
        vi.stubGlobal('window', {
            matchMedia: () => ({
                matches: true,
                addEventListener: (_: string, listener: (event: { matches: boolean }) => void) =>
                    listeners.push(listener)
            })
        });
        const { layout, panel } = load();
        expect(layout.isOverlay()).toBe(true);
        expect(layout.isHidden()).toBe(true);

        layout.show();
        expect(layout.isHidden()).toBe(false);

        layout.closeOverlayTree();
        expect(layout.isHidden()).toBe(true);

        layout.show();
        layout.hide();
        expect(layout.isHidden()).toBe(true);
        expect(panel.isHidden()).toBe(false);
        expect(saved.has('sidePanelHidden')).toBe(false);

        layout.show();
        listeners.forEach(listener => listener({ matches: false }));
        expect(layout.isOverlay()).toBe(false);
        expect(layout.isHidden()).toBe(false);
    });
});
