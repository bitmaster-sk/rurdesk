import { Injectable, inject } from '@angular/core';
import { Observable, combineLatest, first, map } from 'rxjs';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { StateStore } from 'src/app/state/store/state.store';
import { ReleaseTaskRow } from '../entity/release-task-row.entity';

@Injectable({
    providedIn: 'root'
})
export class ReleaseTaskService {
    private readonly issueApi = inject(IssueApi);
    private readonly issueTypeStore = inject(IssueTypeStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly stateStore = inject(StateStore);

    public loadRows$(idProject: number, idTag: number): Observable<ReleaseTaskRow[]> {
        return combineLatest([
            this.issueApi.load$({
                idProject,
                idsTag: [idTag],
                orderColumn: 'updateAt',
                orderDirection: 'desc'
            }),
            this.issueTypeStore.issueTypesMapByProject$(idProject).pipe(first()),
            this.severityStore.severitiesMapByProject$(idProject).pipe(first()),
            this.stateStore.statesMapByProject$(idProject).pipe(first())
        ]).pipe(
            map(([issues, issueTypes, severities, states]) =>
                issues
                    .filter(issue => issue.idsTag?.includes(idTag))
                    .sort((a, b) => a.idIssuePublic - b.idIssuePublic)
                    .map(issue => ({
                        issue,
                        issueType: issueTypes.get(issue.idIssueType ?? 0) ?? null,
                        severity: severities.get(issue.idSeverity ?? 0) ?? null,
                        state: states.get(issue.idState ?? 0) ?? null
                    }))
            )
        );
    }
}
