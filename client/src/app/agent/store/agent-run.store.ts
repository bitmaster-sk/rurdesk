import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { EMPTY, Observable, Subject, catchError, of, switchMap } from 'rxjs';
import { AgentRunApi } from '../api/agent-run.api.service';
import { AgentRun } from '../model/agent-run.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';

@Injectable()
export class AgentRunStore {
    private readonly agentRunApi = inject(AgentRunApi);
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly toast = inject(ToastNotificationService);

    public readonly run = signal<AgentRun | null>(null);
    public readonly isLoading = signal(false);

    private idProject: number | null = null;
    private idIssuePublic: number | null = null;
    private loadedIdIssue: number | null = null;

    private readonly fetch$ = new Subject<void>();

    constructor() {
        this.fetch$
            .pipe(
                switchMap(() => this.fetchOnce$()),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(run => {
                this.run.set(run);
                this.isLoading.set(false);
            });

        // The backend sends a self-contained snapshot (run + events) in the
        // payload, so no HTTP refetch is needed. The notice is broadcast to
        // every connected user; patch only when:
        //  - we already show a run and the incoming idRun matches, OR
        //  - we don't have one yet but this run belongs to the issue we
        //    were loaded for (covers the first event before initial fetch
        //    completes, or a fresh run inserted by issue creation).
        this.noticeService.agentRun$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(notice => {
            const incoming = notice.payload as AgentRun | null;
            if (!incoming) {
                return;
            }
            const current = this.run();
            if (current) {
                if (incoming.idRun !== current.idRun) {
                    return;
                }
            } else if (this.loadedIdIssue === null || incoming.idIssue !== this.loadedIdIssue) {
                return;
            }
            this.run.set(incoming);
        });
    }

    public loadForIssue(idProject: number, idIssuePublic: number, idIssue: number): void {
        this.idProject = idProject;
        this.idIssuePublic = idIssuePublic;
        this.loadedIdIssue = idIssue;
        this.fetch$.next();
    }

    public approve(mockupRef?: string): void {
        this.act$(run => this.agentRunApi.approve$(run.idRun, mockupRef));
    }

    public cancel(): void {
        this.act$(run => this.agentRunApi.cancel$(run.idRun));
    }

    public continue(): void {
        const current = this.run();
        if (!current) {
            return;
        }
        this.agentRunApi
            .continue$(current.idRun)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.refetchIfLoaded());
    }

    public restart(): void {
        const current = this.run();
        if (!current) {
            return;
        }
        this.agentRunApi
            .restart$(current.idRun)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: () => this.refetchIfLoaded(),
                error: (err: HttpErrorResponse) => {
                    // 409 = the run already has a PR (guard): restarting would open a
                    // duplicate. Tell the user to use Continue instead. Other failures get a
                    // generic notice (the global interceptor only toasts 503). Refetch either
                    // way so the card reflects the true server state.
                    const key =
                        err?.status === 409 ? 'AGENT.RESTART_HAS_PR' : 'AGENT.RESTART_FAILED';
                    this.toast.showError(key);
                    this.refetchIfLoaded();
                }
            });
    }

    private fetchOnce$(): Observable<AgentRun | null> {
        if (this.idProject === null || this.idIssuePublic === null) {
            return EMPTY;
        }
        this.isLoading.set(true);
        return this.agentRunApi.getRunByIssue$(this.idProject, this.idIssuePublic).pipe(
            // A transient fetch failure must not blank an already-shown run —
            // keep the current value. A real "no run" is a 204 (null body),
            // which is a success and still clears the panel.
            catchError(() => of(this.run()))
        );
    }

    private refetchIfLoaded(): void {
        if (this.idProject !== null && this.idIssuePublic !== null) {
            this.fetch$.next();
        }
    }

    private act$(action: (run: AgentRun) => Observable<AgentRun>): void {
        const current = this.run();
        if (!current) {
            return;
        }
        action(current)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(updated => this.run.set(updated));
    }
}
