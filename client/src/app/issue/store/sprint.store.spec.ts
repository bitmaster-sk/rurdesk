import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { Injector, runInInjectionContext } from '@angular/core';
import { of, firstValueFrom, Subject, throwError } from 'rxjs';
import { first } from 'rxjs/operators';
import { SprintStore } from './sprint.store';
import { SprintState } from '../constants/sprint-state.enum';
import { SprintApi } from '../api/sprint.api.service';
import { Sprint } from '../model/sprint.model';

function makeSprint(over: Partial<Sprint>): Sprint {
    return {
        idSprint: 1,
        idProject: 1,
        name: 'Sprint 1',
        startAt: '',
        endAt: '',
        state: SprintState.Planned,
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

    function controllable(): {
        store: SprintStore;
        respond: (idProject: number, sprints: Sprint[]) => void;
    } {
        const responses = new Map([
            [1, new Subject<Sprint[]>()],
            [2, new Subject<Sprint[]>()]
        ]);
        const api = { loadByProject$: vi.fn((idProject: number) => responses.get(idProject)!) };
        const injector = Injector.create({ providers: [{ provide: SprintApi, useValue: api }] });
        return {
            store: runInInjectionContext(injector, () => new SprintStore()),
            respond: (idProject, sprints) => responses.get(idProject)!.next(sprints)
        };
    }

    it('shows no cycles at all between a project switch and that project response', () => {
        const { store, respond } = controllable();
        const seen: (number | undefined)[] = [];
        store.currentSprint$.subscribe(s => seen.push(s?.idSprint));

        store.load(1);
        respond(1, [makeSprint({ idSprint: 11, idProject: 1 })]);
        store.load(2);

        expect(seen.at(-1)).toBeUndefined();

        respond(2, [makeSprint({ idSprint: 22, idProject: 2 })]);

        expect(seen.at(-1)).toBe(22);
    });

    it('scopes a board only from a fresh response, never from the cached list', () => {
        const { store, respond } = controllable();
        store.load(1);
        respond(1, [makeSprint({ idSprint: 11, idProject: 1 })]);

        const scoped: (number | undefined)[] = [];
        store.currentSprintOnLoad$.pipe(first()).subscribe(s => scoped.push(s?.idSprint));
        store.load(2);

        expect(scoped).toEqual([]);

        respond(2, [makeSprint({ idSprint: 22, idProject: 2 })]);

        expect(scoped).toEqual([22]);
    });

    it('falls back to the backlog when the sprint list cannot be loaded', () => {
        const api = { loadByProject$: vi.fn(() => throwError(() => new Error('down'))) };
        const injector = Injector.create({ providers: [{ provide: SprintApi, useValue: api }] });
        const store = runInInjectionContext(injector, () => new SprintStore());

        const scoped: (number | undefined)[] = [];
        store.currentSprintOnLoad$.pipe(first()).subscribe(s => scoped.push(s?.idSprint));
        store.load(1);

        expect(scoped).toEqual([undefined]);
    });

    it('ignores a slow response for a project the user has already left', () => {
        const { store, respond } = controllable();
        const seen: number[][] = [];
        store.sprints$.subscribe(s => seen.push(s.map(x => x.idSprint)));

        store.load(1);
        store.load(2);
        respond(1, [makeSprint({ idSprint: 11, idProject: 1 })]);
        respond(2, [makeSprint({ idSprint: 22, idProject: 2 })]);

        expect(seen.at(-1)).toEqual([22]);
        expect(seen).not.toContainEqual([11]);
    });
});

describe('SprintStore.currentSprint$', () => {
    const s = (
        idSprint: number,
        startAt: string,
        endAt: string,
        state: SprintState = SprintState.Planned
    ): Sprint => makeSprint({ idSprint, startAt, endAt, state });

    beforeEach(() => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date('2026-03-10T12:00:00Z'));
    });

    afterEach(() => vi.useRealTimers());

    async function currentOf(sprints: Sprint[]): Promise<Sprint | undefined> {
        const { store } = makeStore(sprints);
        store.load(1);
        return firstValueFrom(store.currentSprint$);
    }

    it('overlapping windows → earliest startAt wins, tie-break lowest idSprint', async () => {
        const current = await currentOf([
            s(2, '2026-03-09', '2026-03-20'),
            s(1, '2026-03-01', '2026-03-15')
        ]);
        expect(current?.idSprint).toBe(1);
    });

    it('nothing in-window → falls back to the earliest planned sprint', async () => {
        const current = await currentOf([
            s(3, '2026-04-01', '2026-04-14'),
            s(2, '2026-03-20', '2026-04-01')
        ]);
        expect(current?.idSprint).toBe(2);
    });

    it('all closed or empty → undefined', async () => {
        expect(await currentOf([])).toBeUndefined();
        expect(
            await currentOf([s(1, '2026-03-01', '2026-03-15', SprintState.Closed)])
        ).toBeUndefined();
    });

    it('early-closed sprint mid-window is skipped → next planned becomes current', async () => {
        const current = await currentOf([
            s(1, '2026-03-01', '2026-03-15', SprintState.Closed),
            s(2, '2026-03-15', '2026-03-29')
        ]);
        expect(current?.idSprint).toBe(2);
    });
});
