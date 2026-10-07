import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { User } from 'src/app/auth/model/user.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { ActivatedRoute, Router } from '@angular/router';
import { catchError, filter, map, merge, of, switchMap } from 'rxjs';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiCalloutConverter } from '../../converter/wiki-callout.converter';
import { WikiLinkConverter } from '../../converter/wiki-link.converter';
import { WikiTreeConverter } from '../../converter/wiki-tree.converter';
import { WikiPageView } from '../../model/wiki-page.model';
import { WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiLayoutStore } from '../../store/wiki-layout.store';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-page',
    templateUrl: './wiki-page.page.html',
    styleUrls: ['./wiki-page.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiPagePage {
    private readonly route = inject(ActivatedRoute);
    private readonly translate = inject(I18nService);
    private readonly router = inject(Router);
    private readonly api = inject(WikiApi);
    private readonly clipboard = inject(ClipboardService);
    private readonly toast = inject(ToastNotificationService);
    protected readonly store = inject(WikiTreeStore);
    protected readonly layout = inject(WikiLayoutStore);

    private readonly calloutLabels = WikiCalloutConverter.toLabels(key =>
        this.translate.instant(key)
    );

    protected readonly AgentAccess = WikiAgentAccess;
    protected readonly isMoveOpen = signal(false);
    protected readonly isNotFound = signal(false);
    protected readonly view = signal<WikiPageView | null>(null);

    protected readonly idProject = toSignal(
        this.route.paramMap.pipe(map(params => Number(params.get('idProject')))),
        { initialValue: Number(this.route.snapshot.paramMap.get('idProject')) }
    );

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    protected readonly spaceKind = computed(() => this.view()?.spaceKind ?? WikiSpaceKind.Project);

    protected readonly isHome = computed(
        () => !!this.view() && this.store.projectSpace()?.idHomePage === this.view()?.page.idPage
    );

    protected readonly canSetHome = computed(
        () => !!this.view()?.canManage && this.spaceKind() === WikiSpaceKind.Project
    );

    protected readonly renderedBody = computed(() => {
        const view = this.view();
        if (!view) {
            return '';
        }
        return WikiLinkConverter.toMarkdown(
            WikiCalloutConverter.toHtml(view.page.body, this.calloutLabels),
            {
                idProject: this.idProject(),
                links: view.links,
                sharedIdSpace: this.store.sharedSpace()?.idSpace ?? null
            }
        );
    });

    protected readonly breadcrumbs = computed<WikiTreeNode[]>(() => {
        const view = this.view();
        const tree = this.store.tree();
        if (!view || !tree) {
            return [];
        }
        const byId = new Map(tree.nodes.map(node => [node.idPage, node]));
        return WikiTreeConverter.ancestorIds(tree.nodes, view.page.idPage)
            .reverse()
            .map(id => byId.get(id))
            .filter((node): node is WikiTreeNode => !!node);
    });

    protected readonly spaceNodes = computed<WikiTreeNode[]>(() => {
        const view = this.view();
        return (this.store.tree()?.nodes ?? []).filter(node => node.idSpace === view?.page.idSpace);
    });

    protected readonly authorName = computed(() => {
        const idUser = this.view()?.page.updateBy;
        return idUser ? (this.usersMap().get(idUser)?.name ?? '') : '';
    });

    public constructor() {
        const routeLoads = this.route.paramMap.pipe(
            map(params => ({
                space: params.get('space') as WikiSpaceKind,
                slug: params.get('slug') ?? ''
            }))
        );
        const noticeLoads = inject(NoticeService).wikiPage$.pipe(
            filter(notice => notice.payload.idPage === this.view()?.page.idPage),
            map(() => ({ space: this.spaceKind(), slug: this.view()?.page.slug ?? '' }))
        );
        merge(routeLoads, noticeLoads)
            .pipe(
                switchMap(target =>
                    this.api
                        .loadOne$(this.idProject(), target.space, target.slug)
                        .pipe(catchError(() => of(null)))
                ),
                takeUntilDestroyed()
            )
            .subscribe(view => {
                this.isNotFound.set(view === null);
                this.view.set(view);
            });
    }

    protected pageLink(node: { idSpace: number; slug: string }): (string | number)[] {
        return [
            '/project',
            this.idProject(),
            'wiki',
            this.store.kindOfSpace(node.idSpace),
            node.slug
        ];
    }

    protected onBodyClick(event: MouseEvent): void {
        const anchor = (event.target as HTMLElement | null)?.closest('a');
        const href = anchor?.getAttribute('href');
        if (!href || event.metaKey || event.ctrlKey) {
            return;
        }
        if (href.startsWith('#')) {
            event.preventDefault();
            void this.router.navigate([], { relativeTo: this.route, fragment: href.slice(1) });
            return;
        }
        if (!href.startsWith('/')) {
            return;
        }
        event.preventDefault();
        void this.router.navigateByUrl(href);
    }

    protected onToggleHome(): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api
            .updateHome$(this.idProject(), this.isHome() ? null : view.page.idPage)
            .subscribe(() => this.store.reload());
    }

    protected async onCopyLink(): Promise<void> {
        const isCopied = await this.clipboard.copy(window.location.href);
        if (isCopied) {
            this.toast.showSuccess('WIKI.PAGE.LINK_COPIED');
        }
    }

    protected onMove(idParent: number | null): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api
            .move$(view.page.idPage, { idParent, idPrev: null, idNext: null })
            .subscribe(() => this.store.reload());
    }

    protected onDelete(): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api.delete$(view.page.idPage).subscribe(() => {
            this.toast.showSuccess('WIKI.TRASH.MOVED');
            this.store.reload();
            void this.router.navigate(['/project', this.idProject(), 'wiki']);
        });
    }
}
