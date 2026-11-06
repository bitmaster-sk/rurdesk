import { Component, input, model } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, RouterModule, convertToParamMap, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { EMPTY, Observable, Subject, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentRenderDirective } from 'src/app/shared/attachment/directives/attachment-render.directive';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiPageAsideComponent } from '../../components/wiki-page-aside/wiki-page-aside.component';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiHeadingAnchorsDirective } from '../../directives/wiki-heading-anchors.directive';
import { WikiMermaidDirective } from '../../directives/wiki-mermaid.directive';
import {
    WikiBacklinkList,
    WikiPageIssue,
    WikiPageIssueList,
    WikiPageRef,
    WikiPageView
} from '../../model/wiki-page.model';
import { WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiPagePage } from './wiki-page.page';

@Component({ selector: 'app-wiki-tree-show', template: '', standalone: true })
class WikiTreeShowStub {}

@Component({ selector: 'app-wiki-move-dialog', template: '', standalone: true })
class WikiMoveDialogStub {
    public readonly visible = model(false);
    public readonly nodes = input<WikiTreeNode[]>([]);
    public readonly idSpace = input<number>(0);
    public readonly idPage = input<number>(0);
    public readonly idParent = input<number | null>(null);
    public readonly rootLabel = input('');
}

function issue(idIssue: number): WikiPageIssue {
    return {
        idIssue,
        idIssuePublic: idIssue,
        idProject: 7,
        title: `Task ${idIssue}`,
        stateName: 'Open',
        isClosed: false
    };
}

function backlink(idPage: number): WikiPageRef {
    return { idPage, idSpace: 2, slug: `page-${idPage}`, title: `Page ${idPage}` };
}

function issues(from: number, count: number): WikiPageIssue[] {
    return Array.from({ length: count }, (_, index) => issue(from + index));
}

function backlinks(from: number, count: number): WikiPageRef[] {
    return Array.from({ length: count }, (_, index) => backlink(from + index));
}

function pageView(issues: WikiPageIssueList, backlinks: WikiBacklinkList): WikiPageView {
    return {
        page: {
            idPage: 40,
            idSpace: 2,
            idParent: null,
            slug: 'setup',
            title: 'Setup',
            summary: '',
            body: '## Setup\n\nThe page explains how to install the tracker.\n\nKeep reading for details.',
            agentAccess: WikiAgentAccess.OnDemand,
            rank: 'a',
            versionNo: 1,
            deletedAt: null,
            createAt: '2026-10-08T10:00:00Z',
            updateAt: '2026-10-08T10:00:00Z',
            createBy: 1,
            updateBy: 1
        },
        spaceKind: WikiSpaceKind.Project,
        canEdit: false,
        canManage: false,
        backlinks,
        issues,
        links: [],
        draft: null
    };
}

describe('WikiPagePage layout', () => {
    let moreIssues: Subject<WikiPageIssueList>;
    let moreBacklinks: Subject<WikiBacklinkList>;
    let fixture: ComponentFixture<WikiPagePage>;

    beforeEach(() => {
        localStorage.removeItem('wiki.fullWidth');
        localStorage.removeItem('wiki.treeHidden');
        moreIssues = new Subject<WikiPageIssueList>();
        moreBacklinks = new Subject<WikiBacklinkList>();
    });

    afterEach(() => {
        localStorage.removeItem('wiki.fullWidth');
        localStorage.removeItem('wiki.treeHidden');
    });

    // The shell gives the page whatever width is left after the app menu and
    // the wiki tree, so the layout reacts to the width of the host itself:
    // with the tree hidden this equals the viewport, which is also how the
    // page behaves on its own here (the tree is out of scope for this fix).
    function setWidth(width: number): void {
        const host = fixture.nativeElement as HTMLElement;
        host.style.display = 'block';
        host.style.width = `${width}px`;
    }

    function byTestId(testId: string): HTMLElement[] {
        return Array.from(
            (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>(
                `[data-testid="${testId}"]`
            )
        );
    }

    function articleRect(): DOMRect {
        const host = fixture.nativeElement as HTMLElement;
        return host
            .querySelector<HTMLElement>('[data-testid="wiki-page-body"]')!
            .getBoundingClientRect();
    }

    function asideEl(): HTMLElement {
        return (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(
            '.wiki-page__aside'
        )!;
    }

    function asideRect(): DOMRect {
        return asideEl().getBoundingClientRect();
    }

    function hostRect(): DOMRect {
        return (fixture.nativeElement as HTMLElement).getBoundingClientRect();
    }

    async function render(
        issueList: WikiPageIssueList,
        backlinkList: WikiBacklinkList,
        width: number
    ): Promise<void> {
        await TestBed.configureTestingModule({
            declarations: [
                WikiPagePage,
                WikiPageAsideComponent,
                WikiHeadingAnchorsDirective,
                WikiMermaidDirective,
                AttachmentRenderDirective
            ],
            imports: [
                MarkdownModule.forRoot(),
                TranslateModule.forRoot(),
                RouterModule,
                UiModule,
                TablerIconStub,
                WikiTreeShowStub,
                WikiMoveDialogStub
            ],
            providers: [
                provideRouter([]),
                {
                    provide: ActivatedRoute,
                    useValue: {
                        paramMap: of(
                            convertToParamMap({ idProject: '7', space: 'project', slug: 'setup' })
                        ),
                        fragment: EMPTY,
                        snapshot: { paramMap: convertToParamMap({ idProject: '7' }) }
                    }
                },
                {
                    provide: WikiApi,
                    useValue: {
                        loadOne$: () => of(pageView(issueList, backlinkList)),
                        loadPageIssues$: () => moreIssues,
                        loadBacklinks$: () => moreBacklinks
                    }
                },
                {
                    provide: WikiTreeStore,
                    useValue: {
                        tree: () => null,
                        sharedSpace: () => null,
                        kindOfSpace: () => WikiSpaceKind.Project
                    }
                },
                { provide: NoticeService, useValue: { wikiPage$: EMPTY } },
                { provide: ProjectMemberStore, useValue: { usersMap$: of(new Map()) } },
                {
                    provide: ToastNotificationService,
                    useValue: { showError: vi.fn(), showSuccess: vi.fn() }
                },
                {
                    provide: AttachmentApi,
                    useValue: { download$: (): Observable<Blob> => of(new Blob(['x'])) }
                }
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(WikiPagePage);
        setWidth(width);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
    }

    it('stacks the linked panel under the full-width article on narrow screens', async () => {
        await render({ items: issues(1, 2), total: 2 }, { items: backlinks(1, 1), total: 1 }, 1280);

        for (const width of [320, 375, 430]) {
            setWidth(width);
            const article = articleRect();
            const aside = asideRect();
            const bounds = hostRect();

            // The article keeps the full available width and the panel lands
            // below it, inside the viewport — not beside the text.
            expect(article.width).toBeCloseTo(width, 0);
            expect(aside.top).toBeGreaterThanOrEqual(article.bottom - 1);
            expect(aside.right).toBeLessThanOrEqual(bounds.right + 1);
            // The panel is no longer forced to viewport-ish height next to a
            // short page.
            expect(getComputedStyle(asideEl()).minHeight).toBe('0px');
        }

        expect(byTestId('wiki-page-issue')[0].getAttribute('href')).toBe('/project/7/issue/1');
        expect(byTestId('wiki-page-backlink')[0].getAttribute('href')).toBe(
            '/project/7/wiki/project/page-1'
        );
    });

    it('keeps the two-panel desktop layout and the full-width toggle', async () => {
        await render({ items: issues(1, 2), total: 2 }, { items: backlinks(1, 1), total: 1 }, 1280);

        const article = articleRect();
        const aside = asideRect();
        expect(Math.abs(aside.top - article.top)).toBeLessThanOrEqual(1);
        expect(aside.left).toBeGreaterThanOrEqual(article.right - 4);
        expect(aside.width).toBeLessThan(300);
        expect(article.width).toBeGreaterThan(850);

        byTestId('wiki-full-width')[0].click();
        fixture.detectChanges();

        expect(articleRect().width).toBeGreaterThan(article.width);
    });

    it('loads the next tasks and backlinks from the panel stacked under the article', async () => {
        await render(
            { items: issues(1, 20), total: 25 },
            { items: backlinks(1, 20), total: 21 },
            375
        );

        expect(asideRect().top).toBeGreaterThanOrEqual(articleRect().bottom - 1);
        expect(byTestId('wiki-page-issue')).toHaveLength(20);
        expect(byTestId('wiki-page-issues-count')[0].textContent?.trim()).toBe('20 / 25');

        byTestId('wiki-page-issues-more')[0].click();
        fixture.detectChanges();
        moreIssues.next({ items: issues(21, 5), total: 25 });
        fixture.detectChanges();

        expect(byTestId('wiki-page-issue')).toHaveLength(25);
        expect(byTestId('wiki-page-issues-count')[0].textContent?.trim()).toBe('25 / 25');
        expect(byTestId('wiki-page-issues-more')).toEqual([]);

        byTestId('wiki-page-backlinks-more')[0].click();
        fixture.detectChanges();
        moreBacklinks.next({ items: backlinks(21, 1), total: 21 });
        fixture.detectChanges();

        expect(byTestId('wiki-page-backlink')).toHaveLength(21);
        expect(byTestId('wiki-page-backlinks-count')[0].textContent?.trim()).toBe('21 / 21');
        expect(byTestId('wiki-page-backlinks-more')).toEqual([]);
    });
});
