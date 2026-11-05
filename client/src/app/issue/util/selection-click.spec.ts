import { describe, expect, it } from 'vitest';
import { SelectionClick } from './selection-click';

const click = (init: Partial<MouseEvent> & { insideLink?: boolean }): MouseEvent =>
    ({
        metaKey: false,
        ctrlKey: false,
        shiftKey: false,
        ...init,
        target: { closest: () => (init.insideLink ? {} : null) }
    }) as unknown as MouseEvent;

describe('SelectionClick.modeOf', () => {
    it('Cmd or Ctrl click toggles', () => {
        expect(SelectionClick.modeOf(click({ metaKey: true }))).toBe('toggle');
        expect(SelectionClick.modeOf(click({ ctrlKey: true }))).toBe('toggle');
    });

    it('Shift click selects a range', () => {
        expect(SelectionClick.modeOf(click({ shiftKey: true }))).toBe('range');
    });

    it('a plain click does not select', () => {
        expect(SelectionClick.modeOf(click({}))).toBeNull();
    });

    it('leaves clicks on links and buttons to the browser, so Cmd+click still opens a new tab', () => {
        expect(SelectionClick.modeOf(click({ metaKey: true, insideLink: true }))).toBeNull();
        expect(SelectionClick.modeOf(click({ shiftKey: true, insideLink: true }))).toBeNull();
    });
});
