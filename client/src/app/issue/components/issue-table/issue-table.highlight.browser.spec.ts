import { describe, it, expect, beforeEach } from 'vitest';
import { createTableFixture, makeIssue } from './table-testbed.helper';

function makeRow(idIssuePublic: number) {
    return { issue: makeIssue({ idIssuePublic }) };
}

describe('IssueTableComponent highlight (TestBed)', () => {
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        comp = result.comp;
        // Set up 3 rows for highlight tests
        comp.rows.set([makeRow(1), makeRow(2), makeRow(3)]);
    });

    // =========================================================================
    // moveHighlight
    // =========================================================================

    describe('moveHighlight', () => {
        it('delta +1 from null selects first task (index 0)', () => {
            comp.moveHighlight(1);
            expect(comp.highlightedId()).toBe(1);
        });

        it('delta -1 from null selects last task', () => {
            comp.moveHighlight(-1);
            expect(comp.highlightedId()).toBe(3);
        });

        it('delta +1 advances to next task', () => {
            comp.highlightedId.set(1);
            comp.moveHighlight(1);
            expect(comp.highlightedId()).toBe(2);
        });

        it('delta -1 moves to previous task', () => {
            comp.highlightedId.set(3);
            comp.moveHighlight(-1);
            expect(comp.highlightedId()).toBe(2);
        });

        it('delta +1 at last index clamps (stays at last)', () => {
            comp.highlightedId.set(3);
            comp.moveHighlight(1);
            expect(comp.highlightedId()).toBe(3);
        });

        it('delta -1 at index 0 clamps (stays at first)', () => {
            comp.highlightedId.set(1);
            comp.moveHighlight(-1);
            expect(comp.highlightedId()).toBe(1);
        });

        it('empty rows: no-op', () => {
            comp.rows.set([]);
            comp.highlightedId.set(null);
            comp.moveHighlight(1);
            expect(comp.highlightedId()).toBeNull();
        });

        it('tracks by stable idIssuePublic, not index', () => {
            // Simulate: task #2 was at index 1, then list re-ordered → now at index 0
            comp.rows.set([makeRow(2), makeRow(1), makeRow(3)]);
            comp.highlightedId.set(2);
            // Move down → should select task #1 (now at index 1)
            comp.moveHighlight(1);
            expect(comp.highlightedId()).toBe(1);
        });
    });

    // =========================================================================
    // highlightedIssue computed
    // =========================================================================

    describe('highlightedIssue', () => {
        it('returns the issue at the highlighted index', () => {
            comp.highlightedId.set(2);
            expect(comp.highlightedIssue()?.idIssuePublic).toBe(2);
        });

        it('returns null when highlightedId is null', () => {
            comp.highlightedId.set(null);
            expect(comp.highlightedIssue()).toBeNull();
        });

        it('returns null when id is out of range', () => {
            comp.highlightedId.set(999);
            expect(comp.highlightedIssue()).toBeNull();
        });
    });

    // =========================================================================
    // onOpenHighlighted
    // =========================================================================

    it('navigates to the issue detail page', () => {
        const issue = makeIssue({ idProject: 7, idIssuePublic: 42 });
        expect(() => comp.onOpenHighlighted(issue)).not.toThrow();
    });
});
