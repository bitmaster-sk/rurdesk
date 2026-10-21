import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentRunApi } from '../api/agent-run.api.service';
import { RunStats } from '../model/agent-run.model';
import { AgentRunStatsStore } from './agent-run-stats.store';

const STATS: RunStats = {
    totalTokensUsed: 130,
    totalDurationMs: 5000,
    totalToolCallsCount: 4,
    attemptsPerStage: { design: 1 },
    failedAttempts: 0,
    tokensByModel: { 'openai/kimi': 130 }
};

function build(): {
    store: AgentRunStatsStore;
    notices: Subject<Notice<unknown>>;
    loadStats: ReturnType<typeof vi.fn>;
    loadWikiReads: ReturnType<typeof vi.fn>;
} {
    const notices = new Subject<Notice<unknown>>();
    const loadStats = vi.fn().mockReturnValue(of(STATS));
    const loadWikiReads = vi.fn().mockReturnValue(of([]));
    const injector = Injector.create({
        providers: [
            {
                provide: AgentRunApi,
                useValue: { loadStats$: loadStats, loadWikiReads$: loadWikiReads }
            },
            { provide: NoticeService, useValue: { agentStats$: notices.asObservable() } },
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } }
        ]
    });
    const store = runInInjectionContext(injector, () => new AgentRunStatsStore());
    return { store, notices, loadStats, loadWikiReads };
}

function statsNotice(idRun: number, stats?: RunStats): Notice<unknown> {
    return {
        subject: NoticeSubject.AgentStats,
        action: NoticeAction.Update,
        payload: { idRun, stats }
    };
}

describe('AgentRunStatsStore', () => {
    it('loads the stats and wiki reads of the run', () => {
        const { store, loadStats, loadWikiReads } = build();

        store.reload(5);

        expect(loadStats).toHaveBeenCalledWith(5);
        expect(loadWikiReads).toHaveBeenCalledWith(5);
        expect(store.stats()).toEqual(STATS);
    });

    it('takes the stats from a notice of its run and reloads the wiki reads', () => {
        const { store, notices, loadWikiReads } = build();
        store.reload(5);
        const fresher = { ...STATS, totalTokensUsed: 900 };

        notices.next(statsNotice(5, fresher));

        expect(store.stats()?.totalTokensUsed).toBe(900);
        expect(loadWikiReads).toHaveBeenCalledTimes(2);
    });

    it('ignores the notices of another run', () => {
        const { store, notices, loadWikiReads } = build();
        store.reload(5);

        notices.next(statsNotice(99, { ...STATS, totalTokensUsed: 900 }));

        expect(store.stats()?.totalTokensUsed).toBe(130);
        expect(loadWikiReads).toHaveBeenCalledTimes(1);
    });
});
