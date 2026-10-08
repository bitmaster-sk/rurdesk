import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    inject,
    input,
    linkedSignal,
    signal,
    WritableSignal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Observable, finalize } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { WikiApi } from '../../api/wiki.api.service';
import { WIKI_LIST_PAGE_SIZE } from '../../constants/wiki-list.constants';
import {
    WikiBacklinkList,
    WikiPageIssue,
    WikiPageIssueList,
    WikiPageRef
} from '../../model/wiki-page.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-page-aside',
    templateUrl: './wiki-page-aside.component.html',
    styleUrls: ['./wiki-page-aside.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiPageAsideComponent {
    public readonly idProject = input.required<number>();
    public readonly idPage = input.required<number>();
    public readonly issues = input.required<WikiPageIssueList>();
    public readonly backlinks = input.required<WikiBacklinkList>();

    private readonly api = inject(WikiApi);
    private readonly store = inject(WikiTreeStore);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly issueList = linkedSignal(() => this.issues());
    protected readonly backlinkList = linkedSignal(() => this.backlinks());
    protected readonly isLoadingIssues = signal(false);
    protected readonly isLoadingBacklinks = signal(false);

    protected readonly hasIssuePages = computed(() => this.issueList().total > WIKI_LIST_PAGE_SIZE);
    protected readonly canLoadMoreIssues = computed(
        () => this.issueList().items.length < this.issueList().total
    );
    protected readonly hasBacklinkPages = computed(
        () => this.backlinkList().total > WIKI_LIST_PAGE_SIZE
    );
    protected readonly canLoadMoreBacklinks = computed(
        () => this.backlinkList().items.length < this.backlinkList().total
    );

    protected issueLink(issue: WikiPageIssue): (string | number)[] {
        return ['/project', issue.idProject, 'issue', issue.idIssuePublic];
    }

    protected backlinkLink(ref: WikiPageRef): (string | number)[] {
        return [
            '/project',
            this.idProject(),
            'wiki',
            this.store.kindOfSpace(ref.idSpace),
            ref.slug
        ];
    }

    protected onLoadMoreIssues(): void {
        this.loadMore(
            this.isLoadingIssues,
            () => this.api.loadPageIssues$(this.idPage(), this.issueList().items.length),
            page =>
                this.issueList.update(current => ({
                    items: [
                        ...current.items,
                        ...page.items.filter(
                            issue => !current.items.some(known => known.idIssue === issue.idIssue)
                        )
                    ],
                    total: page.total
                }))
        );
    }

    protected onLoadMoreBacklinks(): void {
        this.loadMore(
            this.isLoadingBacklinks,
            () =>
                this.api.loadBacklinks$(
                    this.idProject(),
                    this.idPage(),
                    this.backlinkList().items.length
                ),
            page =>
                this.backlinkList.update(current => ({
                    items: [
                        ...current.items,
                        ...page.items.filter(
                            ref => !current.items.some(known => known.idPage === ref.idPage)
                        )
                    ],
                    total: page.total
                }))
        );
    }

    private loadMore<T>(
        isLoading: WritableSignal<boolean>,
        request: () => Observable<T>,
        append: (page: T) => void
    ): void {
        if (isLoading()) {
            return;
        }
        const idPage = this.idPage();
        isLoading.set(true);
        request()
            .pipe(
                finalize(() => isLoading.set(false)),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe({
                next: page => {
                    if (this.idPage() === idPage) {
                        append(page);
                    }
                },
                error: () => this.toast.showError('WIKI.PAGE.LOAD_MORE_ERROR')
            });
    }
}
