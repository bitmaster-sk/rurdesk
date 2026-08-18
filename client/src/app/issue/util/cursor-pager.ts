import { signal } from '@angular/core';
import { Observable } from 'rxjs';
import { IssuesPage } from '../model/issues-page.model';
import { Issue } from '../model/issue.model';

// CursorPager drives one keyset-paginated list: it accumulates pages and exposes signals.
// "Load more" visibility is driven by nextCursor (non-null), not a separate hasMore flag.
export class CursorPager {
    public readonly items = signal<Issue[]>([]);
    public readonly total = signal(0);
    public readonly isLoading = signal(false);
    private readonly cursor = signal<string | null>(null);

    // Bumped by every request that invalidates the ones before it. A response
    // carrying a stale generation is dropped: without this, a `loadMore` still in
    // flight when the filter or sort changes lands after the fresh first page and
    // appends rows from the old query, then overwrites the cursor with a stale one.
    private generation = 0;

    private readonly fetch: (cursor: string | null, limit?: number) => Observable<IssuesPage>;

    public constructor(fetch: (cursor: string | null, limit?: number) => Observable<IssuesPage>) {
        this.fetch = fetch;
    }

    // Keep the current items visible while the first page reloads (filter/sort
    // change) — `fetchPage(true)` replaces them wholesale on arrival. Clearing
    // to [] here made the table flash empty and its auto-layout columns collapse
    // then jump back. `isLoading` is available for a non-destructive indicator.
    public reset(): void {
        this.cursor.set(null);
        this.fetchPage(true);
    }

    // Re-fetch the pages already loaded as a single request so a data refresh keeps the
    // user's scroll extent instead of snapping back to page 1. Falls back to a normal
    // reset when nothing is loaded yet.
    public refetchExtent(): void {
        const loaded = this.items().length;
        if (loaded === 0) {
            this.reset();
            return;
        }
        const generation = ++this.generation;
        this.isLoading.set(true);
        this.fetch(null, loaded).subscribe({
            next: page => {
                if (generation !== this.generation) {
                    return;
                }
                this.items.set(page.items);
                this.cursor.set(page.nextCursor);
                this.total.set(page.total);
                this.isLoading.set(false);
            },
            error: () => this.finishIfCurrent(generation)
        });
    }

    public loadMore(): void {
        if (this.cursor() === null) {
            return;
        }
        this.fetchPage(false);
    }

    public canLoadMore(): boolean {
        return this.cursor() !== null;
    }

    private fetchPage(isFirst: boolean): void {
        const generation = ++this.generation;
        this.isLoading.set(true);
        this.fetch(isFirst ? null : this.cursor()).subscribe({
            next: page => {
                if (generation !== this.generation) {
                    return;
                }
                this.items.update(current => (isFirst ? page.items : [...current, ...page.items]));
                this.cursor.set(page.nextCursor);
                this.total.set(page.total);
                this.isLoading.set(false);
            },
            error: () => this.finishIfCurrent(generation)
        });
    }

    // Only the request still in charge may clear the spinner; a late error from a
    // superseded one would otherwise stop the spinner of its replacement. Leaving
    // isLoading set disables "Load more" until the view remounts.
    private finishIfCurrent(generation: number): void {
        if (generation === this.generation) {
            this.isLoading.set(false);
        }
    }
}
