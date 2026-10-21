import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Subject, catchError, filter, of, switchMap } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentRunApi } from '../api/agent-run.api.service';
import { AgentStatsNotice, RunStats } from '../model/agent-run.model';
import { AgentRunWikiRead } from '../model/agent-run-wiki-read.model';

@Injectable()
export class AgentRunStatsStore {
    private readonly api = inject(AgentRunApi);
    private readonly noticeService = inject(NoticeService);
    private readonly destroyRef = inject(DestroyRef);

    public readonly stats = signal<RunStats | null>(null);
    public readonly wikiReads = signal<AgentRunWikiRead[]>([]);

    private idRun: number | null = null;
    private readonly statsReload$ = new Subject<number>();
    private readonly wikiReadsReload$ = new Subject<number>();

    public constructor() {
        this.statsReload$
            .pipe(
                switchMap(idRun => this.api.loadStats$(idRun).pipe(catchError(() => of(null)))),
                filter((stats): stats is RunStats => stats !== null),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(stats => this.stats.set(stats));

        this.wikiReadsReload$
            .pipe(
                switchMap(idRun => this.api.loadWikiReads$(idRun).pipe(catchError(() => of(null)))),
                filter((reads): reads is AgentRunWikiRead[] => reads !== null),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(reads => this.wikiReads.set(reads));

        this.noticeService.agentStats$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => {
                const payload = notice.payload as AgentStatsNotice | null;
                if (this.idRun === null || payload?.idRun !== this.idRun) {
                    return;
                }
                if (payload.stats) {
                    this.stats.set(payload.stats);
                }
                this.wikiReadsReload$.next(this.idRun);
            });
    }

    public reload(idRun: number): void {
        if (this.idRun !== idRun) {
            this.stats.set(null);
            this.wikiReads.set([]);
        }
        this.idRun = idRun;
        this.statsReload$.next(idRun);
        this.wikiReadsReload$.next(idRun);
    }
}
