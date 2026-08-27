import { DebugElement } from '@angular/core';
import { By } from '@angular/platform-browser';

import { createTableFixture, makeIssue } from './table-testbed.helper';
import { IssueTableRow } from './entity/issue-table-row.entity';

function row(over: Partial<IssueTableRow> = {}): IssueTableRow {
    return {
        issue: makeIssue(),
        state: undefined,
        severity: undefined,
        issueType: undefined,
        assigned: undefined,
        relations: [],
        ...over
    };
}

describe('IssueTableComponent empty values', () => {
    it('marks an unset type, severity and state with a dash instead of leaving cells blank', async () => {
        const { fixture, mocks } = await createTableFixture();
        mocks.issueTableServiceMock.rows.set([row()]);
        fixture.detectChanges();

        const cells: DebugElement[] = fixture.debugElement.queryAll(
            By.css('tbody td app-empty-value')
        );

        expect(cells.length).toBe(3);
        cells.forEach(cell => expect(cell.nativeElement.textContent).toContain('—'));
    });

    it('renders the value itself once it is set, with no dash left behind', async () => {
        const { fixture, mocks } = await createTableFixture();
        mocks.issueTableServiceMock.rows.set([
            row({
                issueType: {
                    idIssueType: 1,
                    idProject: 5,
                    name: 'BUG',
                    protected: false,
                    orderRank: 1
                },
                severity: {
                    idSeverity: 1,
                    idProject: 5,
                    title: 'High',
                    color: '#f00',
                    protected: false,
                    orderRank: 1
                },
                state: {
                    idState: 1,
                    idProject: 5,
                    name: 'NEW',
                    start: true,
                    final: false,
                    protected: false,
                    orderRank: 1
                }
            })
        ]);
        fixture.detectChanges();

        const body = fixture.debugElement.query(By.css('tbody'));

        expect(fixture.debugElement.queryAll(By.css('tbody td app-empty-value')).length).toBe(0);
        expect(body.nativeElement.textContent).toContain('High');
    });

    it('keeps the dash aligned with the severity value it stands in for', async () => {
        const { fixture, mocks } = await createTableFixture();
        mocks.issueTableServiceMock.rows.set([row()]);
        fixture.detectChanges();

        const dash = fixture.debugElement.query(By.css('tbody .empty-value--dot'));

        expect(parseFloat(getComputedStyle(dash.nativeElement).paddingLeft)).toBeGreaterThan(0);
    });
});
