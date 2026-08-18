import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { Subject, of, throwError } from 'rxjs';
import { AgentRunStore } from './agent-run.store';
import { AgentRunApi } from '../api/agent-run.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AgentRun } from '../model/agent-run.model';
import { AgentPhase } from '../model/agent-phase.enum';

function makeRun(idRun: number, idIssue = 10): AgentRun {
    return {
        idRun,
        idIssue,
        idProject: 1,
        idUserBot: 2,
        idGitIntegration: null,
        phase: AgentPhase.Queued,
        stagePlan: { stages: [] },
        queuePosition: null,
        prUrl: null,
        prHostType: null,
        prId: null,
        branchName: null,
        errorMessage: null,
        startedAt: null,
        finishedAt: null,
        createdAt: '2026-01-01T00:00:00Z'
    };
}

interface ApiMock {
    getRunByIssue$?: ReturnType<typeof vi.fn>;
    approve$?: ReturnType<typeof vi.fn>;
    restart$?: ReturnType<typeof vi.fn>;
}

function build(
    api: ApiMock,
    agentRun$: Subject<{ payload: AgentRun | null }>,
    toast: { showError: ReturnType<typeof vi.fn> } = { showError: vi.fn() }
): AgentRunStore {
    const injector = Injector.create({
        providers: [
            { provide: AgentRunApi, useValue: api },
            { provide: NoticeService, useValue: { agentRun$ } },
            { provide: ToastNotificationService, useValue: toast },
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } }
        ]
    });
    return runInInjectionContext(injector, () => new AgentRunStore());
}

describe('AgentRunStore', () => {
    it('loadForIssue fetches the run and clears loading', () => {
        const run = makeRun(5);
        const getRunByIssue$ = vi.fn().mockReturnValue(of(run));
        const store = build({ getRunByIssue$ }, new Subject());

        store.loadForIssue(1, 2, 10);

        expect(getRunByIssue$).toHaveBeenCalledWith(1, 2);
        expect(store.run()).toBe(run);
        expect(store.isLoading()).toBe(false);
    });

    it('approve sends the current run id and stores the updated run', () => {
        const updated = makeRun(5);
        const api: ApiMock = {
            getRunByIssue$: vi.fn().mockReturnValue(of(makeRun(5))),
            approve$: vi.fn().mockReturnValue(of(updated))
        };
        const store = build(api, new Subject());
        store.loadForIssue(1, 2, 10);

        store.approve();

        expect(api.approve$).toHaveBeenCalledWith(5, undefined);
        expect(store.run()).toBe(updated);
    });

    it('approve passes the chosen mockup ref to the api', () => {
        const updated = makeRun(5);
        const api: ApiMock = {
            getRunByIssue$: vi.fn().mockReturnValue(of(makeRun(5))),
            approve$: vi.fn().mockReturnValue(of(updated))
        };
        const store = build(api, new Subject());
        store.loadForIssue(1, 2, 10);

        store.approve('Mockup B');

        expect(api.approve$).toHaveBeenCalledWith(5, 'Mockup B');
    });

    it('patches the run from a notice with a matching idRun', () => {
        const agentRun$ = new Subject<{ payload: AgentRun | null }>();
        const store = build({ getRunByIssue$: vi.fn().mockReturnValue(of(makeRun(5))) }, agentRun$);
        store.loadForIssue(1, 2, 10);

        const patched = makeRun(5);
        agentRun$.next({ payload: patched });
        expect(store.run()).toBe(patched);
    });

    it('ignores a notice for a different run', () => {
        const agentRun$ = new Subject<{ payload: AgentRun | null }>();
        const initial = makeRun(5);
        const store = build({ getRunByIssue$: vi.fn().mockReturnValue(of(initial)) }, agentRun$);
        store.loadForIssue(1, 2, 10);

        agentRun$.next({ payload: makeRun(99) });
        expect(store.run()).toBe(initial);
    });

    it('restart on a run with a PR (409) shows the has-PR toast and refetches', () => {
        const getRunByIssue$ = vi.fn().mockReturnValue(of(makeRun(5)));
        const restart$ = vi.fn().mockReturnValue(throwError(() => ({ status: 409 })));
        const toast = { showError: vi.fn() };
        const store = build({ getRunByIssue$, restart$ }, new Subject(), toast);
        store.loadForIssue(1, 2, 10);
        getRunByIssue$.mockClear();

        store.restart();

        expect(restart$).toHaveBeenCalledWith(5);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.RESTART_HAS_PR');
        expect(getRunByIssue$).toHaveBeenCalledTimes(1); // refetch to resync the card
    });

    it('restart failure (non-409) shows the generic toast', () => {
        const getRunByIssue$ = vi.fn().mockReturnValue(of(makeRun(5)));
        const restart$ = vi.fn().mockReturnValue(throwError(() => ({ status: 500 })));
        const toast = { showError: vi.fn() };
        const store = build({ getRunByIssue$, restart$ }, new Subject(), toast);
        store.loadForIssue(1, 2, 10);

        store.restart();

        expect(toast.showError).toHaveBeenCalledWith('AGENT.RESTART_FAILED');
    });

    it('restart success refetches and shows no toast', () => {
        const getRunByIssue$ = vi.fn().mockReturnValue(of(makeRun(5)));
        const restart$ = vi.fn().mockReturnValue(of({ oldIdRun: 5, newIdRun: 6 }));
        const toast = { showError: vi.fn() };
        const store = build({ getRunByIssue$, restart$ }, new Subject(), toast);
        store.loadForIssue(1, 2, 10);
        getRunByIssue$.mockClear();

        store.restart();

        expect(toast.showError).not.toHaveBeenCalled();
        expect(getRunByIssue$).toHaveBeenCalledTimes(1);
    });
});
