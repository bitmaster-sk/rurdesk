import { describe, expect, it } from 'vitest';
import { resolveHighlightIndex } from './highlight.util';

const row = (idIssuePublic: number) => ({ issue: { idIssuePublic } });

describe('resolveHighlightIndex', () => {
    it('returns null when nothing is selected', () => {
        expect(resolveHighlightIndex([row(1), row(2)], null)).toBeNull();
    });

    it('finds the index of the selected task', () => {
        expect(resolveHighlightIndex([row(10), row(20), row(30)], 20)).toBe(1);
    });

    it('follows the task when the list is re-ordered (the reported bug)', () => {
        // Task #20 is selected at index 1, then an edit bumps it to the front.
        const before = [row(10), row(20), row(30)];
        const after = [row(20), row(10), row(30)];
        expect(resolveHighlightIndex(before, 20)).toBe(1);
        // Same selected id, new order → highlight moves with the task, not the slot.
        expect(resolveHighlightIndex(after, 20)).toBe(0);
    });

    it('clears the highlight when the selected task is gone (e.g. filtered out)', () => {
        expect(resolveHighlightIndex([row(10), row(30)], 20)).toBeNull();
    });

    it('returns null for an empty list', () => {
        expect(resolveHighlightIndex([], 20)).toBeNull();
    });
});
