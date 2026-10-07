import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    inject,
    input,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Subscription, debounceTime, filter } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { WikiIssueLinkSource } from 'src/app/wiki/constants/wiki-issue-link-source.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiTreeConverter } from 'src/app/wiki/converter/wiki-tree.converter';
import { WikiPagePick } from 'src/app/wiki/entity/wiki-page-pick.entity';
import { WikiIssueLink } from 'src/app/wiki/model/wiki-page.model';
import { WikiTree } from 'src/app/wiki/model/wiki-tree.model';

@Component({
    selector: 'app-issue-wiki-panel',
    templateUrl: './issue-wiki-panel.component.html',
    styleUrls: ['./issue-wiki-panel.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueWikiPanelComponent {
    public readonly idProject = input.required<number>();
    public readonly idIssue = input<number | null>(null);
    public readonly canEdit = input(false);

    private readonly api = inject(WikiApi);
    private treeRequest: Subscription | null = null;

    protected readonly Source = WikiIssueLinkSource;
    protected readonly SpaceKind = WikiSpaceKind;
    protected readonly links = signal<WikiIssueLink[]>([]);
    protected readonly isCollapsed = signal(true);
    protected readonly isPickerOpen = signal(false);
    protected readonly tree = signal<WikiTree | null>(null);
    protected readonly query = signal('');

    protected readonly pickGroups = computed(() => {
        const tree = this.tree();
        if (!tree) {
            return [];
        }
        return WikiTreeConverter.toPickGroups(
            tree,
            this.query(),
            this.links().map(link => link.idPage)
        );
    });

    public constructor() {
        effect(() => {
            const idIssue = this.idIssue();
            if (idIssue !== null) {
                this.load(idIssue);
            }
        });

        inject(NoticeService)
            .issue$.pipe(
                filter(notice => notice.payload.idIssue === this.idIssue()),
                debounceTime(300),
                takeUntilDestroyed()
            )
            .subscribe(() => {
                const idIssue = this.idIssue();
                if (idIssue !== null) {
                    this.load(idIssue);
                }
            });
    }

    protected link(page: WikiIssueLink): (string | number)[] {
        return ['/project', this.idProject(), 'wiki', page.spaceKind, page.slug];
    }

    protected onToggle(): void {
        this.isCollapsed.update(isCollapsed => !isCollapsed);
    }

    protected onPickerToggle(): void {
        if (this.isPickerOpen()) {
            return;
        }
        this.isPickerOpen.set(true);
        this.query.set('');
        this.tree.set(null);
        this.treeRequest?.unsubscribe();
        this.treeRequest = this.api.loadTree$(this.idProject()).subscribe({
            next: tree => this.tree.set(tree),
            error: () =>
                this.tree.set({
                    spaces: [],
                    nodes: [],
                    alwaysTokens: 0,
                    tokenLimit: 0,
                    trashCount: 0
                })
        });
    }

    protected onPickerClosed(): void {
        this.isPickerOpen.set(false);
        this.treeRequest?.unsubscribe();
    }

    protected onFilter(event: Event): void {
        if (event.target instanceof HTMLInputElement) {
            this.query.set(event.target.value);
        }
    }

    protected onAdd(page: WikiPagePick): void {
        const idIssue = this.idIssue();
        if (idIssue === null || page.isLinked) {
            return;
        }
        this.api.addIssueLink$(idIssue, page.idPage).subscribe(() => {
            this.isCollapsed.set(false);
            this.load(idIssue);
        });
    }

    protected onRemove(page: WikiIssueLink): void {
        const idIssue = this.idIssue();
        if (idIssue === null) {
            return;
        }
        this.api.removeIssueLink$(idIssue, page.idPage).subscribe(() => this.load(idIssue));
    }

    private load(idIssue: number): void {
        this.api.loadIssueLinks$(idIssue).subscribe(links => this.links.set(links));
    }
}
