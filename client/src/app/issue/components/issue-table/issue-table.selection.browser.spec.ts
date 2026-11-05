import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Subject } from 'rxjs';
import { IssueTableComponent } from './issue-table.component';
import {
    configureTableTestBed,
    createTableFixture,
    makeIssue,
    TableMocks
} from './table-testbed.helper';
import { IssueSelectionStore } from '../../store/issue-selection.store';
import { ListSelectionHandler } from 'src/app/core/command/hotkey.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';

function rowsOf(...ids: number[]) {
    return ids.map(idIssuePublic => ({ issue: makeIssue({ idIssuePublic }) }));
}

describe('IssueTableComponent selection (TestBed)', () => {
    let fixture: any;
    let comp: any;
    let mocks: TableMocks;

    const selection = (): IssueSelectionStore =>
        fixture.debugElement.injector.get(IssueSelectionStore);

    const rowEl = (idIssuePublic: number): HTMLTableRowElement =>
        fixture.nativeElement.querySelector(`tr[data-flip-id="${idIssuePublic}"]`);

    const clickCell = (idIssuePublic: number, init: MouseEventInit): void => {
        rowEl(idIssuePublic)
            .querySelector('td.id-col')!
            .dispatchEvent(new MouseEvent('click', { bubbles: true, ...init }));
        fixture.detectChanges();
    };

    const handler = (): ListSelectionHandler =>
        mocks.hotkeysMock.registerSelectionHandler.mock.calls[0][0];

    beforeEach(async () => {
        localStorage.clear();
        const result = await createTableFixture();
        fixture = result.fixture;
        comp = result.comp;
        mocks = result.mocks;
        mocks.issueTableServiceMock.hasLoaded.set(true);
        comp.rows.set(rowsOf(1, 2, 3, 4));
        fixture.detectChanges();
    });

    it('Cmd+click on a row selects it and a second Cmd+click deselects it', () => {
        clickCell(2, { metaKey: true });
        expect(rowEl(2).getAttribute('aria-selected')).toBe('true');
        expect(rowEl(2).classList).toContain('row--selected');

        clickCell(2, { ctrlKey: true });
        expect(rowEl(2).getAttribute('aria-selected')).toBe('false');
    });

    it('Shift+click selects every row between the last picked row and the clicked one', () => {
        clickCell(1, { metaKey: true });
        clickCell(3, { shiftKey: true });

        expect(selection().count()).toBe(3);
        expect(rowEl(4).getAttribute('aria-selected')).toBe('false');
    });

    it('a plain click does not select anything', () => {
        clickCell(2, {});
        expect(selection().count()).toBe(0);
    });

    it('keyboard: x toggles the highlighted row and shift+arrow extends the selection', () => {
        comp.highlightedId.set(1);
        handler().toggle!();
        handler().extend!(1);
        handler().extend!(1);
        fixture.detectChanges();

        expect(selection().count()).toBe(3);
        expect(comp.highlightedId()).toBe(3);
    });

    it('keyboard: X selects all loaded rows, e opens bulk edit and Esc clears', () => {
        expect(handler().edit()).toBe(false);

        handler().toggleAll!();
        expect(selection().count()).toBe(4);

        expect(handler().edit()).toBe(true);
        fixture.detectChanges();
        expect(fixture.nativeElement.querySelector('app-bulk-edit-dialog')).not.toBeNull();

        expect(handler().clear()).toBe(true);
        expect(selection().count()).toBe(0);
        expect(handler().clear()).toBe(false);
    });

    it('applying a bulk edit clears the selection, closes the dialog and refreshes the list', () => {
        handler().toggleAll!();
        comp.openBulkEdit();
        comp.onBulkEditApplied([makeIssue({ idIssuePublic: 1 })]);
        fixture.detectChanges();

        expect(selection().count()).toBe(0);
        expect(fixture.nativeElement.querySelector('app-bulk-edit-dialog')).toBeNull();
        expect(mocks.issueFilterStoreMock.refresh).toHaveBeenCalled();
    });

    it('drops a task deleted in this project from the selection and the open bulk edit', () => {
        selection().toggle(makeIssue({ idIssuePublic: 1 }));
        selection().toggle(makeIssue({ idIssuePublic: 2 }));
        comp.openBulkEdit();

        mocks.issueNotice$.next({
            action: NoticeAction.Delete,
            payload: makeIssue({ idIssuePublic: 2, idProject: 10 })
        });
        mocks.issueNotice$.next({
            action: NoticeAction.Delete,
            payload: makeIssue({ idIssuePublic: 1, idProject: 99 })
        });

        expect(
            selection()
                .selected()
                .map(issue => issue.idIssuePublic)
        ).toEqual([1]);
        expect(
            comp.bulkEditIssues().map((issue: { idIssuePublic: number }) => issue.idIssuePublic)
        ).toEqual([1]);
    });
});

describe('IssueTableComponent selection scope (TestBed)', () => {
    it('keeps the selection when only the sort changes and clears it when the filter changes', async () => {
        localStorage.clear();
        const mocks = configureTableTestBed();
        const filter$ = new Subject<any>();
        mocks.issueFilterStoreMock.actualFilter$ = filter$;
        await TestBed.compileComponents();
        const fixture = TestBed.createComponent(IssueTableComponent);
        fixture.detectChanges();
        const store = fixture.debugElement.injector.get(IssueSelectionStore);
        const base = {
            idProject: 10,
            orderColumn: 'idIssue',
            orderDirection: 'desc',
            idsState: [1]
        };

        filter$.next(base);
        store.toggle(makeIssue({ idIssuePublic: 5 }));
        filter$.next({ ...base, orderColumn: 'title' });
        expect(store.count()).toBe(1);

        filter$.next({ ...base, idsState: [1, 2] });
        expect(store.count()).toBe(0);
    });
});
