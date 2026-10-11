import { Component, input, model } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, RouterModule, convertToParamMap, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { EMPTY, Observable, of, throwError } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AttachmentApi } from 'src/app/shared/attachment/api/attachment.api.service';
import { AttachmentLightboxComponent } from 'src/app/shared/attachment/components/attachment-lightbox/attachment-lightbox.component';
import { AttachmentRenderDirective } from 'src/app/shared/attachment/directives/attachment-render.directive';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiHeadingAnchorsDirective } from '../../directives/wiki-heading-anchors.directive';
import { WikiMermaidDirective } from '../../directives/wiki-mermaid.directive';
import { WikiPageView } from '../../model/wiki-page.model';
import { WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiPagePage } from './wiki-page.page';

@Component({ selector: 'app-wiki-tree-show', template: '', standalone: true })
class WikiTreeShowStub {}

@Component({ selector: 'app-wiki-page-aside', template: '', standalone: true })
class WikiPageAsideStub {
    public readonly idProject = input<number>(0);
    public readonly idPage = input<number>(0);
    public readonly issues = input<unknown>(null);
    public readonly backlinks = input<unknown>(null);
}

@Component({ selector: 'app-wiki-move-dialog', template: '', standalone: true })
class WikiMoveDialogStub {
    public readonly visible = model(false);
    public readonly nodes = input<WikiTreeNode[]>([]);
    public readonly idSpace = input<number>(0);
    public readonly idPage = input<number>(0);
    public readonly idParent = input<number | null>(null);
    public readonly rootLabel = input('');
}

const idImage = '11111111-1111-1111-1111-111111111111';
const idImage2 = '22222222-2222-2222-2222-222222222222';
const idFile = '33333333-3333-3333-3333-333333333333';

const blobs = new Map<string, Blob>([
    [idImage, new Blob(['png'], { type: 'image/png' })],
    [idImage2, new Blob(['png2'], { type: 'image/png' })],
    [idFile, new Blob(['x'.repeat(2048)], { type: 'application/zip' })]
]);

function pageView(body: string): WikiPageView {
    return {
        page: {
            idPage: 40,
            idSpace: 2,
            idParent: null,
            slug: 'setup',
            title: 'Setup',
            summary: '',
            body,
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
        backlinks: { items: [], total: 0 },
        issues: { items: [], total: 0 },
        links: [],
        draft: null
    };
}

async function render(body: string): Promise<ComponentFixture<WikiPagePage>> {
    await TestBed.configureTestingModule({
        declarations: [
            WikiPagePage,
            WikiHeadingAnchorsDirective,
            WikiMermaidDirective,
            AttachmentRenderDirective,
            AttachmentLightboxComponent
        ],
        imports: [
            MarkdownModule.forRoot(),
            TranslateModule.forRoot(),
            RouterModule,
            UiModule,
            TablerIconStub,
            WikiTreeShowStub,
            WikiPageAsideStub,
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
            { provide: WikiApi, useValue: { loadOne$: () => of(pageView(body)) } },
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
                provide: AttachmentApi,
                useValue: {
                    download$: (id: string): Observable<Blob> => {
                        const blob = blobs.get(id);
                        return blob ? of(blob) : throwError(() => new Error('404'));
                    }
                }
            }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiPagePage);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
}

async function settle(): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 50));
}

describe('WikiPagePage attachments', () => {
    it('shows an image thumbnail and a file card in the page body', async () => {
        const fixture = await render(
            `![diagram.png](attachment:${idImage})\n\n[data.zip](attachment:${idFile})`
        );
        const body = (fixture.nativeElement as HTMLElement).querySelector(
            '[data-testid="wiki-page-body"]'
        )!;

        expect(
            body.querySelector('[data-testid="attachment-thumb"] img')?.getAttribute('alt')
        ).toBe('diagram.png');
        expect(body.querySelector('[data-testid="attachment-card-name"]')?.textContent).toBe(
            'data.zip'
        );
    });

    it('steps through every image of the page in the lightbox', async () => {
        const fixture = await render(
            `![one.png](attachment:${idImage})\n\n## Later\n\n![two.png](attachment:${idImage2})`
        );
        (fixture.nativeElement as HTMLElement)
            .querySelector<HTMLElement>('[data-testid="attachment-thumb"]')!
            .click();
        await settle();
        fixture.detectChanges();
        const alt = (): string | null | undefined =>
            document.querySelector('[data-testid="ui-lightbox-image"]')?.getAttribute('alt');
        expect(alt()).toBe('one.png');

        document.querySelector<HTMLElement>('[data-testid="ui-lightbox-next"]')!.click();
        await settle();

        expect(alt()).toBe('two.png');
    });
});
