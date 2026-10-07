import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WikiLayoutStore } from './wiki-layout.store';

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
        const store = new WikiLayoutStore();
        expect(store.isFullWidth()).toBe(false);

        store.toggleFullWidth();

        expect(store.isFullWidth()).toBe(true);
        expect(new WikiLayoutStore().isFullWidth()).toBe(true);
    });

    it('shows the page tree until it is hidden and remembers that for the next visit', () => {
        const store = new WikiLayoutStore();
        expect(store.isTreeHidden()).toBe(false);

        store.toggleTree();

        expect(store.isTreeHidden()).toBe(true);
        expect(new WikiLayoutStore().isTreeHidden()).toBe(true);
        expect(new WikiLayoutStore().isFullWidth()).toBe(false);
    });
});
