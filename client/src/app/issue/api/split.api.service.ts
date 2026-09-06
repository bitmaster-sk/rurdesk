import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ProposedIssue, SplitAcceptRes, SplitPreviewRes } from '../model/split.model';
import { RequestContext } from 'src/app/core/request-context';

@Injectable({ providedIn: 'root' })
export class SplitApi {
    private readonly http = inject(HttpClient);

    public preview$(
        idProject: number,
        idIssuePublic: number,
        hint?: string
    ): Observable<SplitPreviewRes> {
        return this.http.post<SplitPreviewRes>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/split`,
            { hint: hint ?? '' },

            { context: RequestContext.disableErrorToast() }
        );
    }

    public accept$(
        idProject: number,
        idIssuePublic: number,
        children: ProposedIssue[]
    ): Observable<SplitAcceptRes> {
        return this.http.post<SplitAcceptRes>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/split/accept`,
            { children },

            { context: RequestContext.disableErrorToast() }
        );
    }
}
