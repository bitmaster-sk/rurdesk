import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, NavigationEnd, Router } from '@angular/router';
import {
    Subject,
    catchError,
    debounceTime,
    distinctUntilChanged,
    filter,
    map,
    of,
    startWith,
    switchMap
} from 'rxjs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiSnippetConverter, WikiSnippetPart } from '../../converter/wiki-snippet.converter';
import { WikiTreeMove } from '../../components/wiki-tree/wiki-tree.component';
import { WikiSearchHit, WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiLayoutStore } from '../../store/wiki-layout.store';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-shell',
    templateUrl: './wiki-shell.page.html',
    styleUrls: ['./wiki-shell.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiShellPage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(WikiApi);
    private readonly destroyRef = inject(DestroyRef);
    protected readonly store = inject(WikiTreeStore);

    protected readonly SpaceKind = WikiSpaceKind;
    protected readonly layout = inject(WikiLayoutStore);
    protected readonly query = signal('');
    protected readonly hits = signal<WikiSearchHit[]>([]);
    private readonly searchedQuery = signal('');
    protected readonly isSearching = computed(() => this.query().trim() !== this.searchedQuery());

    private readonly queries = new Subject<string>();

    protected readonly idProject = toSignal(
        this.route.paramMap.pipe(map(params => Number(params.get('idProject')))),
        { initialValue: Number(this.route.snapshot.paramMap.get('idProject')) }
    );

    private readonly url = toSignal(
        this.router.events.pipe(
            filter(event => event instanceof NavigationEnd),
            map(() => this.router.url),
            startWith(this.router.url)
        ),
        { initialValue: this.router.url }
    );

    protected readonly activeIdPage = computed<number | null>(() => {
        const match = /\/wiki\/(instance|project)\/([^/?#]+)/.exec(this.url());
        const tree = this.store.tree();
        if (!match || !tree) {
            return null;
        }
        const kind = match[1] === 'instance' ? WikiSpaceKind.Instance : WikiSpaceKind.Project;
        const idSpace = tree.spaces.find(space => space.kind === kind)?.idSpace;
        const slug = decodeURIComponent(match[2]);
        return (
            tree.nodes.find(node => node.idSpace === idSpace && node.slug === slug)?.idPage ?? null
        );
    });

    protected readonly canCreate = computed(
        () => !!this.store.projectSpace()?.canEdit || !!this.store.sharedSpace()?.canEdit
    );

    public constructor() {
        this.route.paramMap
            .pipe(
                map(params => Number(params.get('idProject'))),
                distinctUntilChanged(),
                switchMap(idProject => this.store.load$(idProject)),
                takeUntilDestroyed()
            )
            .subscribe();

        this.queries
            .pipe(
                debounceTime(250),
                distinctUntilChanged(),
                switchMap(query => {
                    const trimmed = query.trim();
                    const hits$ = trimmed
                        ? this.api.search$(this.idProject(), query).pipe(catchError(() => of([])))
                        : of([]);
                    return hits$.pipe(map(hits => ({ query: trimmed, hits })));
                }),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(result => {
                this.hits.set(result.hits);
                this.searchedQuery.set(result.query);
            });

        effect(() => {
            const idPage = this.activeIdPage();
            if (idPage !== null) {
                this.store.expandPathTo(idPage);
            }
        });
    }

    protected onSearchInput(event: Event): void {
        if (event.target instanceof HTMLInputElement) {
            this.onSearch(event.target.value);
        }
    }

    protected onSearch(query: string): void {
        this.query.set(query);
        this.queries.next(query);
    }

    protected onClearSearch(): void {
        this.onSearch('');
    }

    protected onCreate(space: WikiSpaceKind): void {
        void this.router.navigate(['/project', this.idProject(), 'wiki', 'new'], {
            queryParams: { space }
        });
    }

    protected onMoved(move: WikiTreeMove): void {
        this.api
            .move$(move.idPage, {
                idParent: move.idParent,
                idPrev: move.idPrev,
                idNext: move.idNext
            })
            .subscribe({ complete: () => this.store.reload(), error: () => this.store.reload() });
    }

    protected hitLink(hit: WikiSearchHit): (string | number)[] {
        return [
            '/project',
            this.idProject(),
            'wiki',
            this.store.kindOfSpace(hit.idSpace),
            hit.slug
        ];
    }

    protected alwaysLink(node: WikiTreeNode): (string | number)[] {
        return [
            '/project',
            this.idProject(),
            'wiki',
            this.store.kindOfSpace(node.idSpace),
            node.slug
        ];
    }

    protected snippetParts(hit: WikiSearchHit): WikiSnippetPart[] {
        return WikiSnippetConverter.toParts(hit.snippet || hit.summary);
    }
}
