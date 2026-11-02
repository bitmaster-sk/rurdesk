import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { Subject, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Issue } from 'src/app/issue/model/issue.model';
import { IssueLinkApi } from 'src/app/link/api/issue-link.api.service';
import { IssueLinkCardComponent } from 'src/app/link/components/issue-link-card/issue-link-card.component';
import { IssueLinkRenderDirective } from 'src/app/link/directives/issue-link-render.directive';
import { IssueLinkInfo } from 'src/app/link/model/issue-link.model';
import { AvatarComponent } from 'src/app/shared/avatar/avatar.component';
import { IssueTypeBadgeComponent } from 'src/app/shared/issue-type-badge/issue-type-badge.component';
import { SeverityBadgeComponent } from 'src/app/shared/severity-badge/severity-badge.component';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { StateBadgeComponent } from 'src/app/shared/state-badge/state-badge.component';
import { UiHoverCard } from 'src/app/ui/service/ui-hover-card.service';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { WikiAgentAccess } from 'src/app/wiki/constants/wiki-agent-access.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiTree } from 'src/app/wiki/model/wiki-tree.model';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { AttachmentLightboxStub, TablerIconStub } from 'src/testing/stubs';
import { MessageBodyComponent } from './message-body.component';

@Component({ selector: 'app-route-stub', template: '', standalone: true })
class RouteStub {}

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

const openState = {
    idState: 1,
    idProject: 7,
    name: 'Open',
    start: true,
    final: false,
    protected: false,
    orderRank: 1
};

const doneState = {
    ...openState,
    idState: 2,
    name: 'Done',
    start: false,
    final: true,
    orderRank: 2
};

const infoFor = (idState: number): IssueLinkInfo => ({
    issues: [
        {
            idProject: 7,
            idIssuePublic: 123,
            title: 'Fix login',
            idState,
            idSeverity: 4,
            idIssueType: 3,
            assignedTo: 5
        }
    ],
    projects: [{ idProject: 7, name: 'Rurdesk' }],
    states: [openState, doneState],
    severities: [
        {
            idSeverity: 4,
            idProject: 7,
            title: 'High',
            color: '#ff0000',
            protected: false,
            orderRank: 1
        }
    ],
    issueTypes: [{ idIssueType: 3, idProject: 7, name: 'Bug', protected: false, orderRank: 1 }],
    users: [{ idUser: 5, name: 'Ann Smith', colorAvatarBg: '#336699' }]
});

const emptyInfo: IssueLinkInfo = {
    issues: [],
    projects: [],
    states: [],
    severities: [],
    issueTypes: [],
    users: []
};

async function render(
    body: string,
    opts: { idProject?: number | null; info?: IssueLinkInfo } = {}
): Promise<{
    fixture: ComponentFixture<MessageBodyComponent>;
    loadTree: ReturnType<typeof vi.fn>;
    loadInfo: ReturnType<typeof vi.fn>;
    notices: Subject<Notice<Issue>>;
}> {
    const loadTree = vi.fn().mockReturnValue(of(tree));
    const loadInfo = vi.fn().mockReturnValue(of(opts.info ?? infoFor(openState.idState)));
    const notices = new Subject<Notice<Issue>>();
    await TestBed.configureTestingModule({
        imports: [
            MarkdownModule.forRoot({
                markedOptions: MARKDOWN_MARKED_OPTIONS,
                markedExtensions: [MARKDOWN_MENTION_EXTENSION]
            }),
            UiModule,
            RouterModule,
            TranslateModule.forRoot(),
            TablerIconStub,
            AttachmentLightboxStub,
            DiffViewerStub
        ],
        declarations: [
            MessageBodyComponent,
            MockupCardComponent,
            IssueLinkRenderDirective,
            IssueLinkCardComponent,
            StateBadgeComponent,
            IssueTypeBadgeComponent,
            SeverityBadgeComponent,
            AvatarComponent
        ],
        providers: [
            provideRouter([{ path: '**', component: RouteStub }]),
            LinkScope,
            { provide: WikiApi, useValue: { loadTree$: loadTree } },
            { provide: IssueLinkApi, useValue: { loadInfo$: loadInfo } },
            { provide: NoticeService, useValue: { issue$: notices } }
        ]
    }).compileComponents();
    TestBed.inject(LinkScope).idProject.set(opts.idProject ?? null);
    const fixture = TestBed.createComponent(MessageBodyComponent);
    fixture.componentRef.setInput('body', body);
    fixture.detectChanges();
    await fixture.whenStable();
    await Promise.resolve();
    fixture.detectChanges();
    return { fixture, loadTree, loadInfo, notices };
}

function links(fixture: ComponentFixture<MessageBodyComponent>): HTMLAnchorElement[] {
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('a'));
}

function issueLinks(fixture: ComponentFixture<MessageBodyComponent>): HTMLAnchorElement[] {
    return Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLAnchorElement>(
            '[data-testid="issue-link"]'
        )
    );
}

function hover(anchor: HTMLElement): void {
    anchor.dispatchEvent(new PointerEvent('pointerover', { bubbles: true, pointerType: 'mouse' }));
}

function leave(anchor: HTMLElement): void {
    anchor.dispatchEvent(new PointerEvent('pointerout', { bubbles: true, pointerType: 'mouse' }));
}

function card(): HTMLElement | null {
    return document.querySelector('[data-testid="issue-link-card"]');
}

async function wait(ms: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, ms));
}

describe('MessageBodyComponent issue links', () => {
    afterEach(() => TestBed.inject(UiHoverCard).close());

    it('shows a stored task link as a chip even in a chat without a project', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123) there');
        expect(
            issueLinks(fixture).map(link => [link.textContent, link.getAttribute('href')])
        ).toEqual([['#123', '/project/7/issue/123']]);
        expect(issueLinks(fixture)[0].classList).toContain('issue-link');
    });

    it('leaves a typed #123 as plain text, even inside a project', async () => {
        const { fixture, loadInfo } = await render('blocked by #123', { idProject: 7 });
        expect(links(fixture)).toEqual([]);
        expect((fixture.nativeElement as HTMLElement).textContent).toContain('blocked by #123');
        expect(loadInfo).not.toHaveBeenCalled();
    });

    it('keeps a link with its own label as a normal link', async () => {
        const { fixture } = await render('see [the login bug](/project/7/issue/123)');
        expect(links(fixture).map(link => link.textContent)).toEqual(['the login bug']);
        expect(issueLinks(fixture)).toEqual([]);
    });

    it('links wiki pages and stored task links together', async () => {
        const { fixture, loadTree } = await render(
            'see [[Deploy runbook]] and [#11](/project/9/issue/11)',
            { idProject: 7 }
        );
        expect(loadTree).toHaveBeenCalledWith(7);
        expect(links(fixture).map(link => [link.textContent, link.getAttribute('href')])).toEqual([
            ['Deploy runbook', '/project/7/wiki/project/deploy-runbook'],
            ['#11', '/project/9/issue/11']
        ]);
    });

    it('loads every task link of the message in one request', async () => {
        const { loadInfo } = await render(
            '[#123](/project/7/issue/123), [#4](/project/9/issue/4) and [#123](/project/7/issue/123)'
        );
        expect(loadInfo).toHaveBeenCalledTimes(1);
        expect(loadInfo).toHaveBeenCalledWith([
            { idProject: 7, idIssuePublic: 123 },
            { idProject: 9, idIssuePublic: 4 }
        ]);
    });

    it('strikes through a link to a closed task', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)', {
            info: infoFor(doneState.idState)
        });
        expect(issueLinks(fixture)[0].classList).toContain('issue-link--final');
    });

    it('strikes through the link as soon as a notice closes the task', async () => {
        const { fixture, notices } = await render('see [#123](/project/7/issue/123)');
        expect(issueLinks(fixture)[0].classList).not.toContain('issue-link--final');

        notices.next({
            subject: NoticeSubject.Issue,
            action: NoticeAction.Update,
            payload: {
                idIssue: 1,
                idIssuePublic: 123,
                idProject: 7,
                idState: doneState.idState,
                idSeverity: 4,
                idIssueType: 3,
                title: 'Fix login',
                description: '',
                tracked: 0,
                assignedTo: 5
            }
        });
        fixture.detectChanges();

        expect(issueLinks(fixture)[0].classList).toContain('issue-link--final');
    });

    it('shows the task details in a card while the mouse rests on the link', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)');
        const anchor = issueLinks(fixture)[0];

        hover(anchor);
        expect(card()).toBeNull();
        await wait(UiHoverCard.openDelay + 50);

        expect(
            card()?.querySelector('[data-testid="issue-link-card-title"]')?.textContent?.trim()
        ).toBe('Fix login');
        expect(
            card()?.querySelector('[data-testid="issue-link-card-project"]')?.textContent?.trim()
        ).toBe('Rurdesk');
        expect(card()?.textContent).toContain('Open');
        expect(card()?.textContent).toContain('Bug');
        expect(
            card()?.querySelector('[data-testid="issue-link-card-severity"]')?.textContent?.trim()
        ).toBe('High');
        expect(
            card()?.querySelector('[data-testid="issue-link-card-number"]')?.textContent?.trim()
        ).toBe('#123');
        expect(
            card()?.querySelector('[data-testid="issue-link-card-assignee"]')?.textContent
        ).toContain('Ann Smith');

        leave(anchor);
        await wait(UiHoverCard.closeDelay + 50);
        expect(card()).toBeNull();
    });

    it('hides the card when the link is clicked', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)');
        const anchor = issueLinks(fixture)[0];
        hover(anchor);
        await wait(UiHoverCard.openDelay + 50);
        expect(card()).not.toBeNull();

        anchor.click();

        expect(card()).toBeNull();
    });

    it('does not open the card after the link was clicked during the delay', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)');
        const anchor = issueLinks(fixture)[0];

        hover(anchor);
        anchor.click();
        await wait(UiHoverCard.openDelay + 50);

        expect(card()).toBeNull();
    });

    it('links the task title in the card to the task and hides the card when it is opened', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)');
        hover(issueLinks(fixture)[0]);
        await wait(UiHoverCard.openDelay + 50);
        const title = card()!.querySelector<HTMLAnchorElement>(
            '[data-testid="issue-link-card-title"]'
        )!;

        expect(title.getAttribute('href')).toBe('/project/7/issue/123');
        title.click();

        expect(card()).toBeNull();
    });

    it('shows no card for a task the reader cannot see', async () => {
        const { fixture } = await render('see [#123](/project/7/issue/123)', { info: emptyInfo });
        const anchor = issueLinks(fixture)[0];

        hover(anchor);
        await wait(UiHoverCard.openDelay + 50);

        expect(card()).toBeNull();
        expect(anchor.classList).not.toContain('issue-link--final');
    });
});
