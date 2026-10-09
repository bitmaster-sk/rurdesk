import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterLink, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, Subject, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TablerIconStub, UiLoaderStub } from 'src/testing/stubs';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { WikiIssueLinkSource } from 'src/app/wiki/constants/wiki-issue-link-source.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiIssueLink } from 'src/app/wiki/model/wiki-page.model';
import { WikiAgentAccess } from 'src/app/wiki/constants/wiki-agent-access.enum';
import { WikiTree, WikiTreeNode } from 'src/app/wiki/model/wiki-tree.model';
import { IssueWikiPanelComponent } from './issue-wiki-panel.component';

@Component({ selector: 'ui-button', template: '<ng-content></ng-content>', standalone: true })
class ButtonStub {
    public readonly variant = input<string>('filled');
    public readonly size = input<string>('default');
    public readonly severity = input<string>('primary');
    public readonly ariaLabel = input<string>('');
}

@Component({
    selector: 'ui-popover',
    template: '<ng-content></ng-content>',
    standalone: true,
    exportAs: 'uiPopover'
})
class PopoverStub {
    public toggle(): void {}
    public hide(): void {}
}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

const links: WikiIssueLink[] = [
    {
        idPage: 10,
        idSpace: 2,
        spaceKind: WikiSpaceKind.Project,
        slug: 'agent-run',
        title: 'Agent run',
        source: WikiIssueLinkSource.Description
    },
    {
        idPage: 11,
        idSpace: 1,
        spaceKind: WikiSpaceKind.Instance,
        slug: 'git',
        title: 'Git integration',
        source: WikiIssueLinkSource.Manual
    }
];

const page = (
    idPage: number,
    idSpace: number,
    idParent: number | null,
    title: string
): WikiTreeNode => ({
    idPage,
    idSpace,
    idParent,
    slug: `p${idPage}`,
    title,
    agentAccess: WikiAgentAccess.OnDemand,
    rank: `${idPage}`
});

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
        page(10, 2, null, 'Agent run'),
        page(12, 2, 10, 'Prehľad stavov'),
        page(11, 1, null, 'Git integration')
    ],
    alwaysTokens: 0,
    tokenLimit: 0,
    trashCount: 0,
    proposalCount: 0
};

async function setup(
    canEdit: boolean,
    trees = new Subject<WikiTree>()
): Promise<{
    fixture: ComponentFixture<IssueWikiPanelComponent>;
    api: {
        loadIssueLinks$: ReturnType<typeof vi.fn>;
        removeIssueLink$: ReturnType<typeof vi.fn>;
        addIssueLink$: ReturnType<typeof vi.fn>;
        loadTree$: ReturnType<typeof vi.fn>;
    };
}> {
    const api = {
        loadIssueLinks$: vi.fn().mockReturnValue(of(links)),
        removeIssueLink$: vi.fn().mockReturnValue(of(undefined)),
        addIssueLink$: vi.fn().mockReturnValue(of(undefined)),
        loadTree$: vi.fn().mockReturnValue(trees)
    };
    await TestBed.configureTestingModule({
        declarations: [IssueWikiPanelComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            ButtonStub,
            PopoverStub,
            UiLoaderStub,
            RouterLink,
            TablerIconStub
        ],
        providers: [
            provideRouter([]),
            { provide: WikiApi, useValue: api },
            { provide: NoticeService, useValue: { issue$: new Subject() } }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(IssueWikiPanelComponent);
    fixture.componentRef.setInput('idProject', 7);
    fixture.componentRef.setInput('idIssue', 42);
    fixture.componentRef.setInput('canEdit', canEdit);
    fixture.detectChanges();
    (fixture.nativeElement as HTMLElement)
        .querySelector<HTMLElement>('.wiki-panel__header')
        ?.click();
    fixture.detectChanges();
    return { fixture, api };
}

describe('IssueWikiPanelComponent', () => {
    it('lists linked pages with their source and links them to the wiki', async () => {
        const { fixture, api } = await setup(false);
        const element = fixture.nativeElement as HTMLElement;
        expect(api.loadIssueLinks$).toHaveBeenCalledWith(42);
        const rows = element.querySelectorAll('[data-testid="issue-wiki-link"]');
        expect(rows.length).toBe(2);
        expect(rows[0].textContent).toContain('WIKI.ISSUE.FROM_DESCRIPTION');
        expect(rows[1].querySelector('a')?.getAttribute('href')).toBe(
            '/project/7/wiki/instance/git'
        );
    });

    it('lets an editor remove only manual links', async () => {
        const { fixture, api } = await setup(true);
        const rows = (fixture.nativeElement as HTMLElement).querySelectorAll(
            '[data-testid="issue-wiki-link"]'
        );
        expect(rows[0].querySelector('ui-button')).toBeNull();
        rows[1].querySelector<HTMLElement>('ui-button')?.click();
        expect(api.removeIssueLink$).toHaveBeenCalledWith(42, 11);
    });

    it('hides editing actions from readers', async () => {
        const { fixture } = await setup(false);
        const element = fixture.nativeElement as HTMLElement;
        expect(element.querySelector('[data-testid="issue-wiki-add"]')).toBeNull();
        expect(element.querySelectorAll('[data-testid="issue-wiki-link"] ui-button').length).toBe(
            0
        );
    });

    it('loads the wiki tree when the picker opens and shows a loader until it arrives', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture, api } = await setup(true, trees);
        const element = fixture.nativeElement as HTMLElement;
        expect(api.loadTree$).not.toHaveBeenCalled();

        element.querySelector<HTMLElement>('[data-testid="issue-wiki-add"]')?.click();
        fixture.detectChanges();
        expect(api.loadTree$).toHaveBeenCalledWith(7);
        expect(element.querySelector('[data-testid="issue-wiki-picker-loading"]')).not.toBeNull();

        trees.next(tree);
        fixture.detectChanges();
        expect(element.querySelector('[data-testid="issue-wiki-picker-loading"]')).toBeNull();
        const picks = Array.from(
            element.querySelectorAll<HTMLButtonElement>('[data-testid="issue-wiki-pick"]')
        );
        expect(picks.map(pick => pick.textContent?.trim())).toEqual([
            'Git integration',
            'Agent run',
            'Prehľad stavov'
        ]);
        expect(picks[0].disabled).toBe(true);
        expect(picks[1].disabled).toBe(true);
        expect(picks[2].disabled).toBe(false);
    });

    it('filters the tree by part of a title and links the picked page', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture, api } = await setup(true, trees);
        const element = fixture.nativeElement as HTMLElement;
        element.querySelector<HTMLElement>('[data-testid="issue-wiki-add"]')?.click();
        trees.next(tree);
        fixture.detectChanges();

        const input = element.querySelector<HTMLInputElement>('[data-testid="issue-wiki-search"]');
        input!.value = 'prehlad';
        input!.dispatchEvent(new Event('input'));
        fixture.detectChanges();

        const picks = element.querySelectorAll<HTMLElement>('[data-testid="issue-wiki-pick"]');
        expect(picks.length).toBe(1);
        picks[0].click();
        expect(api.addIssueLink$).toHaveBeenCalledWith(42, 12);
    });
});
