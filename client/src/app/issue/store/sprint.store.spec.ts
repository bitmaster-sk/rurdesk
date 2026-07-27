import { describe, it, expect, vi } from 'vitest';
import { Injector, runInInjectionContext } from '@angular/core';
import { of, firstValueFrom } from 'rxjs';
import { SprintStore, selectCurrentSprint } from './sprint.store';
import { SprintApi } from '../api/sprint.api.service';
import { Sprint } from '../model/sprint.model';

function makeSprint(over: Partial<Sprint>): Sprint {
    return {
        idSprint: 1,
        idProject: 1,
        name: 'Sprint 1',
        startAt: '',
        endAt: '',
        state: 'planned',
        ...over
    };
}

function makeStore(sprints: Sprint[]): {
    store: SprintStore;
    api: { loadByProject$: ReturnType<typeof vi.fn>; create$: ReturnType<typeof vi.fn> };
} {
    const api = {
        loadByProject$: vi.fn().mockReturnValue(of(sprints)),
        create$: vi.fn().mockReturnValue(of(sprints[0] ?? null))
    };
    const injector = Injector.create({ providers: [{ provide: SprintApi, useValue: api }] });
    const store = runInInjectionContext(injector, () => new SprintStore());
    return { store, api };
}

describe('SprintStore', () => {
    it('loads sprints for a project', async () => {
        const { store, api } = makeStore([makeSprint({ idSprint: 7 })]);
        store.load(1);
        const sprints = await firstValueFrom(store.sprints$);
        expect(sprints.map(s => s.idSprint)).toEqual([7]);
        expect(api.loadByProject$).toHaveBeenCalledWith(1);
    });
});

describe('selectCurrentSprint', () => {
    const now = new Date('2026-03-10T12:00:00Z');
    const s = (
        idSprint: number,
        startAt: string,
        endAt: string,
        state: 'planned' | 'closed' = 'planned'
    ): Sprint => makeSprint({ idSprint, startAt, endAt, state });

    it('overlapping windows → earliest startAt wins, tie-break lowest idSprint', () => {
        const sprints = [s(2, '2026-03-09', '2026-03-20'), s(1, '2026-03-01', '2026-03-15')];
        expect(selectCurrentSprint(sprints, now)?.idSprint).toBe(1);
    });

    it('nothing in-window → falls back to the earliest planned sprint', () => {
        const sprints = [s(3, '2026-04-01', '2026-04-14'), s(2, '2026-03-20', '2026-04-01')];
        expect(selectCurrentSprint(sprints, now)?.idSprint).toBe(2);
    });

    it('all closed or empty → undefined', () => {
        expect(selectCurrentSprint([], now)).toBeUndefined();
        expect(
            selectCurrentSprint([s(1, '2026-03-01', '2026-03-15', 'closed')], now)
        ).toBeUndefined();
    });

    it('early-closed sprint mid-window is skipped → next planned becomes current', () => {
        const sprints = [
            s(1, '2026-03-01', '2026-03-15', 'closed'),
            s(2, '2026-03-15', '2026-03-29')
        ];
        expect(selectCurrentSprint(sprints, now)?.idSprint).toBe(2);
    });
});
