import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Observable } from 'rxjs';
import { IssueParticipantApi } from '../api/issue-participant.api.service';
import { IssueParticipantModel } from '../model/issue-participant.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';

/**
 * Business-logic service for issue participants.
 * Holds the current participant list as a signal and keeps it in sync
 * with WebSocket broadcasts from the backend.
 *
 * Must be provided at component level (not root) so each issue-detail
 * instance has its own isolated state.
 */
@Injectable()
export class IssueParticipantService {
    private readonly api = inject(IssueParticipantApi);
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);

    private readonly _participants = signal<IssueParticipantModel[]>([]);
    // Internal idIssue used for WS-event matching (backend broadcasts with idIssue, not idIssuePublic)
    private _loadedIdIssue: number | null = null;

    public readonly participants = this._participants.asReadonly();

    public constructor() {
        this.noticeService.participant$.pipe(takeUntilDestroyed()).subscribe(notice => {
            if (notice.payload.idIssue === this._loadedIdIssue) {
                this._participants.set(notice.payload.participants);
            }
        });
    }

    /**
     * Loads participants for the given issue.
     * @param idProject     project identifier
     * @param idIssuePublic public issue identifier used in the API URL
     * @param idIssue       internal issue identifier used for WS-event matching
     */
    public load(idProject: number, idIssuePublic: number, idIssue: number): void {
        this._loadedIdIssue = idIssue;
        this._participants.set([]);
        this.api
            .list$(idProject, idIssuePublic)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(list => {
                this._participants.set(list);
            });
    }

    public add$(idProject: number, idIssuePublic: number, idUser: number): Observable<void> {
        return this.api.add$(idProject, idIssuePublic, idUser);
    }

    public setMyNotifications$(
        idProject: number,
        idIssuePublic: number,
        enabled: boolean
    ): Observable<void> {
        return this.api.setNotifications$(idProject, idIssuePublic, enabled);
    }
}
