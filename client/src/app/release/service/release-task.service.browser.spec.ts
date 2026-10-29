import { TestBed } from '@angular/core/testing';
import { firstValueFrom, of } from 'rxjs';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { Fixtures } from 'src/testing/fixtures';
import { describe, expect, it, vi } from 'vitest';
import { ReleaseTaskService } from './release-task.service';

const FEATURE: IssueType = {
    idIssueType: 2,
    idProject: 3,
    name: 'Feature',
    protected: false,
    orderRank: 1
};
const HIGH: IssueSeverity = {
    idSeverity: 4,
    idProject: 3,
    title: 'High',
    color: '#f00',
    protected: false,
    orderRank: 1
};
const CLOSED: IssueState = {
    idState: 6,
    idProject: 3,
    name: 'Closed',
    start: false,
    final: true,
    protected: false,
    orderRank: 3
};

describe('ReleaseTaskService', () => {
    function setup(issues = [Fixtures.issue()]) {
        const load$ = vi.fn().mockReturnValue(of(issues));
        TestBed.configureTestingModule({
            providers: [
                { provide: IssueApi, useValue: { load$ } },
                {
                    provide: IssueTypeStore,
                    useValue: { issueTypesMapByProject$: () => of(new Map([[2, FEATURE]])) }
                },
                {
                    provide: SeverityStore,
                    useValue: { severitiesMapByProject$: () => of(new Map([[4, HIGH]])) }
                },
                {
                    provide: StateStore,
                    useValue: { statesMapByProject$: () => of(new Map([[6, CLOSED]])) }
                }
            ]
        });
        return { service: TestBed.inject(ReleaseTaskService), load$ };
    }

    it('loads the tasks of the tag with their type, severity and state, by number', async () => {
        const { service, load$ } = setup([
            Fixtures.issue({
                idIssue: 2,
                idIssuePublic: 9,
                idsTag: [7],
                idIssueType: null,
                idSeverity: null,
                idState: null
            }),
            Fixtures.issue({
                idIssue: 1,
                idIssuePublic: 4,
                idsTag: [7],
                idIssueType: 2,
                idSeverity: 4,
                idState: 6
            })
        ]);

        const rows = await firstValueFrom(service.loadRows$(3, 7));

        expect(load$).toHaveBeenCalledWith(expect.objectContaining({ idProject: 3, idsTag: [7] }));
        expect(rows.map(row => row.issue.idIssuePublic)).toEqual([4, 9]);
        expect(rows[0]).toMatchObject({ issueType: FEATURE, severity: HIGH, state: CLOSED });
        expect(rows[1]).toMatchObject({ issueType: null, severity: null, state: null });
    });

    it('leaves out tasks without the tag even when the filter got lost on the way', async () => {
        const { service } = setup([
            Fixtures.issue({ idIssue: 1, idIssuePublic: 1, idsTag: [7] }),
            Fixtures.issue({ idIssue: 2, idIssuePublic: 2, idsTag: [8] })
        ]);

        const rows = await firstValueFrom(service.loadRows$(3, 7));

        expect(rows.map(row => row.issue.idIssuePublic)).toEqual([1]);
    });
});
