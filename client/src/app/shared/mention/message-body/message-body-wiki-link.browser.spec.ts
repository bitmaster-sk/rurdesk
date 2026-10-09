import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
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
import { TablerIconStub } from 'src/testing/stubs';
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
        },
        {
            idPage: 11,
            idSpace: 1,
            idParent: null,
            slug: 'security',
            title: 'Security',
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
    idProject: number | null
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
            DiffViewerStub
        ],
        declarations: [MessageBodyComponent, MockupCardComponent],
        providers: [
            provideRouter([]),
            { provide: WikiApi, useValue: { loadTree$: loadTree } },
            WikiLinkScope
        ]
    }).compileComponents();
    TestBed.inject(WikiLinkScope).idProject.set(idProject);
    const fixture = TestBed.createComponent(MessageBodyComponent);
    fixture.componentRef.setInput('body', body);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, loadTree };
}

function links(fixture: ComponentFixture<MessageBodyComponent>): HTMLAnchorElement[] {
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('a'));
}

describe('MessageBodyComponent wiki links', () => {
    it('turns wiki links into links to project, shared and missing pages', async () => {
        const { fixture, loadTree } = await render(
            'read [[Deploy runbook]], [[security]] and [[Nope]]',
            7
        );
        expect(loadTree).toHaveBeenCalledWith(7);
        expect(links(fixture).map(link => [link.textContent, link.getAttribute('href')])).toEqual([
            ['Deploy runbook', '/project/7/wiki/project/deploy-runbook'],
            ['Security', '/project/7/wiki/instance/security'],
            ['Nope', '/project/7/wiki/new?space=project&title=Nope']
        ]);
        expect(links(fixture)[2].getAttribute('title')).toBe('missing');
    });

    it('opens a wiki link inside the app instead of reloading the page', async () => {
        const { fixture } = await render('read [[Deploy runbook]]', 7);
        const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
        links(fixture)[0].click();
        expect(navigate).toHaveBeenCalledWith('/project/7/wiki/project/deploy-runbook');
    });

    it('leaves wiki links as text outside a project and loads nothing', async () => {
        const { fixture, loadTree } = await render('read [[Deploy runbook]]', null);
        expect(loadTree).not.toHaveBeenCalled();
        expect(links(fixture)).toEqual([]);
        expect((fixture.nativeElement as HTMLElement).textContent).toContain('[[Deploy runbook]]');
    });
});
