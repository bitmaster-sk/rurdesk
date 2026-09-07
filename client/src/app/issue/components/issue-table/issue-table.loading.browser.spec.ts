import { describe, it, expect, beforeEach } from 'vitest';
import { createTableFixture, makeIssue } from './table-testbed.helper';

function makeRow(idIssuePublic: number) {
    return { issue: makeIssue({ idIssuePublic }) };
}

describe('IssueTableComponent loading state (TestBed)', () => {
    let fixture: any;
    let comp: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        fixture = result.fixture;
        comp = result.comp;
    });

    function tbodyEl(): HTMLElement {
        return fixture.nativeElement.querySelector('tbody') as HTMLElement;
    }

    function hasLoader(): boolean {
        return tbodyEl()?.querySelector('ui-loader') !== null;
    }

    it('shows a loading row when hasLoaded is false, even if not loading', async () => {
        comp.hasLoaded.set(false);
        comp.isLoading.set(false);
        comp.rows.set([]);
        fixture.detectChanges();

        expect(hasLoader()).toBe(true);
    });

    it('shows a loading row when isLoading and rows are empty', async () => {
        comp.hasLoaded.set(false);
        comp.isLoading.set(true);
        comp.rows.set([]);
        fixture.detectChanges();

        expect(hasLoader()).toBe(true);
    });

    it('does not show a loading row when rows already exist (reload)', async () => {
        comp.hasLoaded.set(true);
        comp.isLoading.set(true);
        comp.rows.set([makeRow(1)]);
        fixture.detectChanges();

        expect(hasLoader()).toBe(false);
    });

    it('shows an empty row when settled (hasLoaded, not loading, no rows)', async () => {
        comp.hasLoaded.set(true);
        comp.isLoading.set(false);
        comp.rows.set([]);
        fixture.detectChanges();

        const emptyRow = tbodyEl().querySelector('.table-empty-row');
        expect(emptyRow).not.toBeNull();
    });

    it('does not show an empty row when hasLoaded is false', async () => {
        comp.hasLoaded.set(false);
        comp.isLoading.set(false);
        comp.rows.set([]);
        fixture.detectChanges();

        const emptyRow = tbodyEl().querySelector('.table-empty-row');
        expect(emptyRow).toBeNull();
    });

    it('uses colspan 8 in normal mode', async () => {
        comp.isRelationMode.set(false);
        comp.hasLoaded.set(false);
        comp.isLoading.set(true);
        comp.rows.set([]);
        fixture.detectChanges();

        const td = tbodyEl().querySelector('.table-loading-row td') as HTMLElement;
        expect(td.getAttribute('colspan')).toBe('8');
    });

    it('uses colspan 10 in relation mode', async () => {
        comp.isRelationMode.set(true);
        comp.hasLoaded.set(false);
        comp.isLoading.set(true);
        comp.rows.set([]);
        fixture.detectChanges();

        const td = tbodyEl().querySelector('.table-loading-row td') as HTMLElement;
        expect(td.getAttribute('colspan')).toBe('10');
    });
});
