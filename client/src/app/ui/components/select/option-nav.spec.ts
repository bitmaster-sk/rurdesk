import { signal } from '@angular/core';
import { describe, expect, it } from 'vitest';
import { UiOptionNav } from './option-nav';

interface Opt {
    label: string;
    id: number;
}

const OPTS: Opt[] = [
    { label: 'Apple', id: 1 },
    { label: 'Banana', id: 2 },
    { label: 'Cherry', id: 3 },
    { label: 'Apricot', id: 4 }
];

function make(getSelectedIndex: () => number = () => -1, now: () => number = () => 0) {
    const options = signal<readonly Opt[]>(OPTS);
    const nav = new UiOptionNav<Opt>(options, o => o.label, getSelectedIndex, now);
    return { nav, options };
}

describe('UiOptionNav', () => {
    it('visibleOptions returns all when no filter', () => {
        const { nav } = make();
        expect(nav.visibleOptions()).toEqual(OPTS);
    });

    it('visibleOptions narrows on filter (case-insensitive, substring)', () => {
        const { nav } = make();
        nav.setFilter('ap');
        expect(nav.visibleOptions().map(o => o.label)).toEqual(['Apple', 'Apricot']);
    });

    it('setFilter resets highlight to first match', () => {
        const { nav } = make();
        nav.setFilter('err'); // Cherry
        expect(nav.highlightedIndex()).toBe(0);
        expect(nav.visibleOptions().map(o => o.label)).toEqual(['Cherry']);
    });

    it('setFilter with no match sets highlight to -1', () => {
        const { nav } = make();
        nav.setFilter('zzz');
        expect(nav.visibleOptions()).toEqual([]);
        expect(nav.highlightedIndex()).toBe(-1);
    });

    it('initHighlight lands on the selected option', () => {
        const { nav } = make(() => 2);
        nav.initHighlight();
        expect(nav.highlightedIndex()).toBe(2);
    });

    it('initHighlight lands on 0 when getSelectedIndex is -1 (multi/listbox)', () => {
        const { nav } = make(() => -1);
        nav.initHighlight();
        expect(nav.highlightedIndex()).toBe(0);
    });

    it('moveHighlight clamps at both ends', () => {
        const { nav } = make();
        nav.initHighlight(); // 0
        nav.moveHighlight(-1);
        expect(nav.highlightedIndex()).toBe(0);
        nav.moveHighlight(1);
        expect(nav.highlightedIndex()).toBe(1);
        nav.moveHighlight(100);
        expect(nav.highlightedIndex()).toBe(OPTS.length - 1);
    });

    it('moveHighlight from -1 goes to first (down) / last (up)', () => {
        const { nav } = make();
        nav.moveHighlight(1);
        expect(nav.highlightedIndex()).toBe(0);
        nav.setHighlight(-1);
        nav.moveHighlight(-1);
        expect(nav.highlightedIndex()).toBe(OPTS.length - 1);
    });

    it('first / last jump to ends', () => {
        const { nav } = make();
        nav.last();
        expect(nav.highlightedIndex()).toBe(OPTS.length - 1);
        nav.first();
        expect(nav.highlightedIndex()).toBe(0);
    });

    it('typeAhead selects by prefix within the 500ms buffer window', () => {
        let t = 1000;
        const { nav } = make(
            () => -1,
            () => t
        );
        nav.typeAhead('b'); // Banana
        expect(nav.visibleOptions()[nav.highlightedIndex()].label).toBe('Banana');
        t = 1200; // within window → 'ap'
        nav.typeAhead('...'); // buffer becomes 'b...' — no match, stays
        // fresh sequence after the window
        t = 3000;
        nav.typeAhead('a'); // Apple (first prefix 'a')
        expect(nav.visibleOptions()[nav.highlightedIndex()].label).toBe('Apple');
    });

    it('reset clears filter, highlight and type-ahead', () => {
        const { nav } = make();
        nav.setFilter('ap');
        nav.setHighlight(1);
        nav.reset();
        expect(nav.filterText()).toBe('');
        expect(nav.highlightedIndex()).toBe(-1);
        expect(nav.visibleOptions()).toEqual(OPTS);
    });

    it('visibleOptions reacts to options() changes', () => {
        const { nav, options } = make();
        options.set([{ label: 'Solo', id: 9 }]);
        expect(nav.visibleOptions().map(o => o.label)).toEqual(['Solo']);
    });
});
