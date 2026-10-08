import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import {
    WikiEditor,
    WikiIssueLink,
    WikiPage,
    WikiPageDraft,
    WikiPageView,
    WikiPageIssueList,
    WikiBacklinkList
} from '../model/wiki-page.model';
import {
    WikiCreateRequest,
    WikiMergePreview,
    WikiMergePreviewRequest,
    WikiMoveRequest,
    WikiSaveRequest,
    WikiSaveResult
} from '../model/wiki-save.model';
import { WikiSearchHit, WikiTrashItem, WikiTree } from '../model/wiki-tree.model';
import { WikiVersion, WikiVersionSummary } from '../model/wiki-version.model';

@Injectable({ providedIn: 'root' })
export class WikiApi {
    private readonly http = inject(HttpClient);

    public loadTree$(idProject: number): Observable<WikiTree> {
        return this.http.get<WikiTree>(`/api/private/project/${idProject}/wiki/tree`);
    }

    public loadOne$(
        idProject: number,
        space: WikiSpaceKind,
        slug: string
    ): Observable<WikiPageView> {
        return this.http.get<WikiPageView>(
            `/api/private/project/${idProject}/wiki/page/${space}/${encodeURIComponent(slug)}`
        );
    }

    public loadPageIssues$(idPage: number, offset: number): Observable<WikiPageIssueList> {
        return this.http.get<WikiPageIssueList>(`/api/private/wiki/page/${idPage}/issues`, {
            params: { offset }
        });
    }

    public loadBacklinks$(
        idProject: number,
        idPage: number,
        offset: number
    ): Observable<WikiBacklinkList> {
        return this.http.get<WikiBacklinkList>(
            `/api/private/project/${idProject}/wiki/backlinks/${idPage}`,
            { params: { offset } }
        );
    }

    public insert$(idProject: number, request: WikiCreateRequest): Observable<WikiPage> {
        return this.http.post<WikiPage>(`/api/private/project/${idProject}/wiki/page`, request);
    }

    public update$(idPage: number, request: WikiSaveRequest): Observable<WikiSaveResult> {
        return this.http.put<WikiSaveResult>(`/api/private/wiki/page/${idPage}`, request, {
            context: RequestContext.disableErrorToast()
        });
    }

    public mergePreview$(
        idPage: number,
        request: WikiMergePreviewRequest
    ): Observable<WikiMergePreview> {
        return this.http.post<WikiMergePreview>(
            `/api/private/wiki/page/${idPage}/merge-preview`,
            request
        );
    }

    public move$(idPage: number, request: WikiMoveRequest): Observable<void> {
        return this.http.put<void>(`/api/private/wiki/page/${idPage}/position`, request);
    }

    public delete$(idPage: number): Observable<void> {
        return this.http.delete<void>(`/api/private/wiki/page/${idPage}`);
    }

    public loadTrash$(idProject: number): Observable<WikiTrashItem[]> {
        return this.http.get<WikiTrashItem[]>(`/api/private/project/${idProject}/wiki/trash`);
    }

    public restore$(idPage: number): Observable<WikiPage> {
        return this.http.post<WikiPage>(`/api/private/wiki/page/${idPage}/restore`, {});
    }

    public purge$(idPage: number): Observable<void> {
        return this.http.delete<void>(`/api/private/wiki/page/${idPage}/purge`);
    }

    public loadVersions$(idPage: number): Observable<WikiVersionSummary[]> {
        return this.http.get<WikiVersionSummary[]>(`/api/private/wiki/page/${idPage}/version`);
    }

    public loadVersion$(idPage: number, versionNo: number): Observable<WikiVersion> {
        return this.http.get<WikiVersion>(`/api/private/wiki/page/${idPage}/version/${versionNo}`);
    }

    public loadDiff$(idPage: number, from: number, to: number): Observable<{ diff: string }> {
        const params = new HttpParams().set('from', from).set('to', to);
        return this.http.get<{ diff: string }>(`/api/private/wiki/page/${idPage}/diff`, { params });
    }

    public revert$(idPage: number, versionNo: number): Observable<WikiPage> {
        return this.http.post<WikiPage>(`/api/private/wiki/page/${idPage}/revert/${versionNo}`, {});
    }

    public loadDraft$(idPage: number): Observable<WikiPageDraft | null> {
        return this.http.get<WikiPageDraft | null>(`/api/private/wiki/page/${idPage}/draft`);
    }

    public saveDraft$(
        idPage: number,
        draft: { baseVersion: number; title: string; summary: string; body: string }
    ): Observable<void> {
        return this.http.put<void>(`/api/private/wiki/page/${idPage}/draft`, draft, {
            context: RequestContext.disableErrorToast()
        });
    }

    public deleteDraft$(idPage: number): Observable<void> {
        return this.http.delete<void>(`/api/private/wiki/page/${idPage}/draft`);
    }

    public heartbeat$(idPage: number): Observable<WikiEditor[]> {
        return this.http.put<WikiEditor[]>(
            `/api/private/wiki/page/${idPage}/editing`,
            {},
            { context: RequestContext.disableErrorToast() }
        );
    }

    public leave$(idPage: number): Observable<WikiEditor[]> {
        return this.http.delete<WikiEditor[]>(`/api/private/wiki/page/${idPage}/editing`, {
            context: RequestContext.disableErrorToast()
        });
    }

    public search$(idProject: number, query: string): Observable<WikiSearchHit[]> {
        const params = new HttpParams().set('q', query);
        return this.http.get<WikiSearchHit[]>(`/api/private/project/${idProject}/wiki/search`, {
            params
        });
    }

    public updateSettings$(idProject: number, alwaysTokenLimit: number): Observable<void> {
        return this.http.put<void>(`/api/private/project/${idProject}/wiki/settings`, {
            alwaysTokenLimit
        });
    }

    public updateHome$(idProject: number, idPage: number | null): Observable<void> {
        return this.http.put<void>(`/api/private/project/${idProject}/wiki/home`, { idPage });
    }

    public loadIssueLinks$(idIssue: number): Observable<WikiIssueLink[]> {
        return this.http.get<WikiIssueLink[]>(`/api/private/issue/${idIssue}/wiki`);
    }

    public addIssueLink$(idIssue: number, idPage: number): Observable<void> {
        return this.http.post<void>(`/api/private/issue/${idIssue}/wiki`, { idPage });
    }

    public removeIssueLink$(idIssue: number, idPage: number): Observable<void> {
        return this.http.delete<void>(`/api/private/issue/${idIssue}/wiki/${idPage}`);
    }
}
