import { Injector, runInInjectionContext } from '@angular/core';
import { Subject } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { IssueLinkStore } from './issue-link.store';

const issue = (idIssuePublic: number, title: string): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 7,
    idState: null,
    idSeverity: null,
    idIssueType: null,
    title,
    description: '',
    tracked: 0
});

function buildStore(): { store: IssueLinkStore; load: ReturnType<typeof vi.fn> } {
    const load = vi.fn().mockImplementation(() => new Subject<Issue[]>());
    const injector = Injector.create({
        providers: [{ provide: IssueApi, useValue: { load$: load } }]
    });
    return { store: runInInjectionContext(injector, () => new IssueLinkStore()), load };
}

describe('IssueLinkStore', () => {
    it('loads the list on first use and serves it from the cache', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        expect(load).toHaveBeenCalledWith({
            idProject: 7,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(store.list(7)).toBeNull();

        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.list(7)).toEqual([{ idIssuePublic: 1, title: 'Fix login' }]);
        expect(store.list(8)).toBeNull();
    });

    it('does not start a second request while the first one is in flight', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        store.ensure(7);
        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(1);
    });

    it('refreshes the list in the background on a later ensure, keeping the old list meanwhile', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);

        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(2);
        expect(store.list(7)).toEqual([{ idIssuePublic: 1, title: 'Fix login' }]);

        load.mock.results[1].value.next([issue(1, 'Fix login'), issue(2, 'Add dark mode')]);
        expect(store.list(7)).toEqual([
            { idIssuePublic: 1, title: 'Fix login' },
            { idIssuePublic: 2, title: 'Add dark mode' }
        ]);
    });

    it('allows a retry after a failed load', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        load.mock.results[0].value.error(new Error('boom'));

        store.ensure(7);
        expect(load).toHaveBeenCalledTimes(2);
        expect(store.list(7)).toBeNull();
    });

    it('keeps projects separate', () => {
        const { store, load } = buildStore();
        store.ensure(7);
        store.ensure(9);
        expect(load).toHaveBeenCalledTimes(2);
        load.mock.results[0].value.next([issue(1, 'Fix login')]);
        expect(store.list(9)).toBeNull();
    });
});
