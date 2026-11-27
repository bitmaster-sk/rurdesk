import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SidePanelStore } from './side-panel.store';

const KEY = 'sidePanelHidden';

/** A fresh store reads the saved state like a reload of the app. */
function load(): SidePanelStore {
    TestBed.resetTestingModule();
    return TestBed.inject(SidePanelStore);
}

describe('SidePanelStore', () => {
    afterEach(() => {
        vi.restoreAllMocks();
        localStorage.removeItem(KEY);
    });

    it('shows the panel by default', () => {
        expect(load().isHidden()).toBe(false);
    });

    it('keeps the panel hidden after a reload', () => {
        load().hide();

        expect(load().isHidden()).toBe(true);
    });

    it('keeps the panel shown after a reload once it is shown again', () => {
        load().hide();
        load().show();

        expect(load().isHidden()).toBe(false);
    });

    it('still hides the panel when the browser refuses storage', () => {
        vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('denied');
        });
        vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('denied');
        });
        const store = load();

        store.hide();

        expect(store.isHidden()).toBe(true);
    });
});
