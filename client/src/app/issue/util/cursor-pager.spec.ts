// @vitest-environment jsdom
import { Subject, of, throwError } from 'rxjs';
import { CursorPager } from './cursor-pager';
import { IssuesPage } from '../model/issues-page.model';
import { Issue } from '../model/issue.model';

const issue = (idIssuePublic: number): Issue => ({
    idIssue: idIssuePublic,
    idIssuePublic,
    idProject: 1,
    idState: null,
    idSeverity: null,
    title: `Issue ${idIssuePublic}`,
    description: '',
    tracked: 0
});

const page = (ids: number[], next: string | null, total: number): IssuesPage => ({
    items: ids.map(issue),
    nextCursor: next,
    total
});

describe('CursorPager', () => {
    it('reset loads page 1 and exposes total + canLoadMore', () => {
        const pager = new CursorPager(() => of(page([1, 2], 'c1', 5)));
        pager.reset();
        expect(pager.items().length).toBe(2);
        expect(pager.total()).toBe(5);
        expect(pager.canLoadMore()).toBe(true);
    });

    it('loadMore appends and stops when nextCursor is null', () => {
        const calls = [page([1, 2], 'c1', 4), page([3, 4], null, 4)];
        let n = 0;
        const pager = new CursorPager(() => of(calls[n++]));
        pager.reset();
        pager.loadMore();
        expect(pager.items().map(i => i.idIssuePublic)).toEqual([1, 2, 3, 4]);
        expect(pager.canLoadMore()).toBe(false);
    });

    it('loadMore is a no-op when there is no cursor', () => {
        let calls = 0;
        const pager = new CursorPager(() => {
            calls++;
            return of(page([1], null, 1));
        });
        pager.reset();
        pager.loadMore();
        expect(calls).toBe(1);
    });

    it('refetchExtent re-requests the loaded count as one page from the start', () => {
        const seen: { cursor: string | null; limit?: number }[] = [];
        const responder = (cursor: string | null, limit?: number) => {
            seen.push({ cursor, limit });
            if (cursor === null && limit === undefined) return of(page([1, 2], 'c1', 4));
            if (cursor === 'c1') return of(page([3, 4], 'c2', 4));
            if (cursor === null && limit === 4) return of(page([1, 2, 3, 5], 'c2b', 4));
            return of(page([], null, 0));
        };
        const pager = new CursorPager(responder);
        pager.reset(); // 2 loaded
        pager.loadMore(); // 4 loaded
        expect(pager.items().length).toBe(4);

        pager.refetchExtent();

        expect(pager.items().map(i => i.idIssuePublic)).toEqual([1, 2, 3, 5]);
        expect(seen).toContainEqual({ cursor: null, limit: 4 });
        expect(pager.canLoadMore()).toBe(true);
    });

    it('refetchExtent falls back to a normal reset when nothing is loaded', () => {
        let calls = 0;
        const pager = new CursorPager(() => {
            calls++;
            return of(page([1], null, 1));
        });
        pager.refetchExtent();
        expect(calls).toBe(1);
        expect(pager.items().length).toBe(1);
    });

    // A stuck isLoading is not a cosmetic spinner problem: the table's "Load more"
    // is gated on it, so one failed request would disable paging until remount.
    it('clears isLoading when a page fails so paging stays usable', () => {
        let fail = true;
        const pager = new CursorPager(() => {
            if (fail) {
                fail = false;
                return throwError(() => new Error('boom'));
            }
            return of(page([1, 2], 'c1', 5));
        });

        pager.reset();
        expect(pager.isLoading()).toBe(false);

        pager.reset();
        expect(pager.items().map(i => i.idIssuePublic)).toEqual([1, 2]);
    });

    // A filter or sort change resets the pager; a loadMore already in flight must
    // not append its old-query rows on top of the fresh first page.
    it('drops a response that a later reset has superseded', () => {
        const slowLoadMore = new Subject<IssuesPage>();
        let call = 0;
        const pager = new CursorPager(cursor => {
            call++;
            if (call === 1) return of(page([1, 2], 'c1', 4));
            if (cursor === 'c1') return slowLoadMore; // in flight
            return of(page([9], null, 1)); // fresh first page after reset
        });

        pager.reset();
        pager.loadMore(); // starts, does not resolve yet
        pager.reset(); // supersedes it — resolves immediately

        expect(pager.items().map(i => i.idIssuePublic)).toEqual([9]);

        slowLoadMore.next(page([3, 4], 'stale-cursor', 4));

        expect(pager.items().map(i => i.idIssuePublic)).toEqual([9]);
        expect(pager.canLoadMore()).toBe(false);
    });
});
