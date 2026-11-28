import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SideMenuStore } from './side-menu.store';

const KEY = 'sideMenuCollapsed';

/** A fresh store reads the saved state like a reload of the app. */
function load(): SideMenuStore {
    TestBed.resetTestingModule();
    return TestBed.inject(SideMenuStore);
}

describe('SideMenuStore', () => {
    afterEach(() => {
        vi.restoreAllMocks();
        localStorage.removeItem(KEY);
    });

    it('starts with the full menu', () => {
        expect(load().isCollapsed()).toBe(false);
    });

    it('keeps the rail after a reload and the full menu once it is opened again', () => {
        load().toggle();

        const reloaded = load();
        expect(reloaded.isCollapsed()).toBe(true);

        reloaded.toggle();
        expect(load().isCollapsed()).toBe(false);
    });

    it('still folds the menu when the browser refuses storage', () => {
        vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('denied');
        });
        vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('denied');
        });
        const store = load();

        store.toggle();

        expect(store.isCollapsed()).toBe(true);
    });
});
