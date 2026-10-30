import { Injectable, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import {
    Observable,
    Subject,
    catchError,
    distinctUntilChanged,
    map,
    of,
    switchMap,
    timer
} from 'rxjs';
import { IssueLinkApi } from '../api/issue-link.api.service';
import { IssueLinkOption } from '../model/issue-link.model';
import { LinkScope } from './link-scope.service';

export interface IssueLinkPickerQuery {
    start: number;
    query: string;
}

interface IssueLinkPickerResults {
    query: string;
    options: IssueLinkOption[];
}

@Injectable()
export class IssueLinkPicker {
    public static readonly searchDelay = 150;
    private static readonly queryPattern = /(^|[\s(])#([\p{L}\p{N}_-]*)$/u;

    private readonly api = inject(IssueLinkApi);
    private readonly linkScope = inject(LinkScope, { optional: true });
    private readonly search = new Subject<string | null>();
    private readonly results = toSignal(
        this.search.pipe(
            distinctUntilChanged(),
            switchMap(query => (query === null ? of(null) : this.search$(query)))
        ),
        { initialValue: null }
    );

    public readonly query = signal<IssueLinkPickerQuery | null>(null);
    public readonly activeIndex = signal(0);
    public readonly isLoading = computed(() => {
        const query = this.query();
        return query !== null && this.results()?.query !== query.query;
    });
    public readonly options = computed<IssueLinkOption[]>(() => {
        const query = this.query();
        const results = this.results();
        return query !== null && results?.query === query.query ? results.options : [];
    });
    public readonly isVisible = computed(
        () => this.query() !== null && (this.isLoading() || this.options().length > 0)
    );

    public detect(textBeforeCaret: string): IssueLinkPickerQuery | null {
        const match = IssueLinkPicker.queryPattern.exec(textBeforeCaret);
        if (!match) {
            this.close();
            return null;
        }
        const next = {
            start: textBeforeCaret.length - match[2].length - 1,
            query: match[2]
        };
        if (this.query()?.query !== next.query) {
            this.activeIndex.set(0);
        }
        this.query.set(next);
        this.search.next(next.query);
        return next;
    }

    public close(): void {
        this.query.set(null);
        this.search.next(null);
    }

    public onKey(key: string): { isHandled: boolean; picked?: IssueLinkOption } {
        if (this.query() === null) {
            return { isHandled: false };
        }
        if (key === 'Escape') {
            this.close();
            return { isHandled: true };
        }
        const options = this.options();
        if (options.length === 0) {
            return { isHandled: false };
        }
        if (key === 'ArrowDown') {
            this.activeIndex.update(index => Math.min(index + 1, options.length - 1));
            return { isHandled: true };
        }
        if (key === 'ArrowUp') {
            this.activeIndex.update(index => Math.max(index - 1, 0));
            return { isHandled: true };
        }
        if (key === 'Enter' || key === 'Tab') {
            return {
                isHandled: true,
                picked: options[Math.min(this.activeIndex(), options.length - 1)]
            };
        }
        return { isHandled: false };
    }

    private search$(query: string): Observable<IssueLinkPickerResults> {
        return timer(IssueLinkPicker.searchDelay).pipe(
            switchMap(() =>
                this.api
                    .load$(query, this.linkScope?.idProject() ?? null)
                    .pipe(catchError(() => of<IssueLinkOption[]>([])))
            ),
            map(options => ({ query, options }))
        );
    }
}
