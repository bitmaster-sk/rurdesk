import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { WikiAgentAccess } from 'src/app/wiki/constants/wiki-agent-access.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiTree } from 'src/app/wiki/model/wiki-tree.model';
import { WikiLinkScope } from 'src/app/wiki/service/wiki-link-scope.service';
import { AttachmentLightboxStub, TablerIconStub } from 'src/testing/stubs';
import { MessageBodyComponent } from './message-body.component';

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

const tree: WikiTree = {
    spaces: [
        {
            idSpace: 1,
            kind: WikiSpaceKind.Instance,
            canEdit: true,
            canManage: false,
            idHomePage: null
        },
        {
            idSpace: 2,
            kind: WikiSpaceKind.Project,
            canEdit: true,
            canManage: true,
            idHomePage: null
        }
    ],
    nodes: [
        {
            idPage: 10,
            idSpace: 2,
            idParent: null,
            slug: 'deploy-runbook',
            title: 'Deploy runbook',
            agentAccess: WikiAgentAccess.OnDemand,
            rank: 'm'
        }
    ],
    alwaysTokens: 0,
    tokenLimit: 0,
    trashCount: 0,
    proposalCount: 0
};

async function render(
    body: string,
    opts: { scopeIdProject?: number | null; inputIdProject?: number | null } = {}
): Promise<{
    fixture: ComponentFixture<MessageBodyComponent>;
    loadTree: ReturnType<typeof vi.fn>;
}> {
    const loadTree = vi.fn().mockReturnValue(of(tree));
    await TestBed.configureTestingModule({
        imports: [
            MarkdownModule.forRoot({
                markedOptions: MARKDOWN_MARKED_OPTIONS,
                markedExtensions: [MARKDOWN_MENTION_EXTENSION]
            }),
            UiModule,
            TranslateModule.forRoot(),
            TablerIconStub,
            AttachmentLightboxStub,
            DiffViewerStub
        ],
        declarations: [MessageBodyComponent, MockupCardComponent],
        providers: [
            provideRouter([]),
            { provide: WikiApi, useValue: { loadTree$: loadTree } },
            WikiLinkScope
        ]
    }).compileComponents();
    TestBed.inject(WikiLinkScope).idProject.set(opts.scopeIdProject ?? null);
    const fixture = TestBed.createComponent(MessageBodyComponent);
    fixture.componentRef.setInput('body', body);
    if (opts.inputIdProject !== null && opts.inputIdProject !== undefined) {
        fixture.componentRef.setInput('idProject', opts.inputIdProject);
    }
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, loadTree };
}

function links(fixture: ComponentFixture<MessageBodyComponent>): HTMLAnchorElement[] {
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('a'));
}

describe('MessageBodyComponent issue links', () => {
    it('turns #123 into an issue link inside a project without loading the wiki', async () => {
        const { fixture, loadTree } = await render('blocked by #123', {
            scopeIdProject: 7
        });
        expect(loadTree).not.toHaveBeenCalled();
        expect(links(fixture).map(link => [link.textContent, link.getAttribute('href')])).toEqual([
            ['#123', '/project/7/issue/123']
        ]);
    });

    it('links several references and uses the idProject input without a scope', async () => {
        const { fixture } = await render('see #9 and #10', { inputIdProject: 9 });
        expect(links(fixture).map(link => link.getAttribute('href'))).toEqual([
            '/project/9/issue/9',
            '/project/9/issue/10'
        ]);
    });

    it('links wiki pages and task references together', async () => {
        const { fixture, loadTree } = await render('see [[Deploy runbook]] and #11', {
            scopeIdProject: 7
        });
        expect(loadTree).toHaveBeenCalledWith(7);
        expect(links(fixture).map(link => [link.textContent, link.getAttribute('href')])).toEqual([
            ['Deploy runbook', '/project/7/wiki/project/deploy-runbook'],
            ['#11', '/project/7/issue/11']
        ]);
    });

    it('leaves task references as text outside a project', async () => {
        const { fixture } = await render('see #123');
        expect(links(fixture)).toEqual([]);
        expect((fixture.nativeElement as HTMLElement).textContent).toContain('see #123');
    });

    it('does not link task references inside code', async () => {
        const { fixture } = await render('try `#1` and\n```\n#2\n```\nthen #3', {
            scopeIdProject: 7
        });
        expect(links(fixture).map(link => link.getAttribute('href'))).toEqual([
            '/project/7/issue/3'
        ]);
        expect((fixture.nativeElement as HTMLElement).textContent).toContain('#2');
    });
});
