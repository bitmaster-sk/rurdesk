import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Component, input, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, Subject, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TablerIconStub, UiButtonStub, UiLoaderStub, UiTooltipStub } from 'src/testing/stubs';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiSearchHit } from '../../model/wiki-tree.model';
import { WikiLayoutStore } from '../../store/wiki-layout.store';
import { WikiTreeStore } from '../../store/wiki-tree.store';
import { WikiShellPage } from './wiki-shell.page';

@Component({ selector: 'app-wiki-tree', template: '', standalone: true })
class WikiTreeStub {
    public readonly entries = input<unknown>();
    public readonly idProject = input<number>();
    public readonly spaceKind = input<unknown>();
    public readonly idHomePage = input<number | null>(null);
    public readonly activeIdPage = input<number | null>(null);
    public readonly expanded = input<unknown>();
    public readonly canEdit = input<boolean>(false);
}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

const hit: WikiSearchHit = {
    idPage: 3,
    idSpace: 2,
    slug: 'deploy',
    title: 'Deploy',
    summary: '',
    snippet: 'how we <<deploy>>',
    score: 1
};

async function render(
    responses: Subject<WikiSearchHit[]>
): Promise<ComponentFixture<WikiShellPage>> {
    await TestBed.configureTestingModule({
        declarations: [WikiShellPage],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            RouterOutlet,
            RouterLink,
            RouterLinkActive,
            TablerIconStub,
            UiButtonStub,
            UiLoaderStub,
            UiTooltipStub,
            WikiTreeStub
        ],
        providers: [
            provideRouter([{ path: '**', children: [] }]),
            { provide: WikiApi, useValue: { search$: () => responses } },
            {
                provide: WikiTreeStore,
                useValue: {
                    tree: signal(null),
                    groups: signal(null),
                    expanded: signal(new Set()),
                    projectSpace: signal(null),
                    sharedSpace: signal(null),
                    load$: () => of(null),
                    kindOfSpace: () => 'project',
                    expandPathTo: () => undefined
                }
            }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiShellPage);
    fixture.detectChanges();
    return fixture;
}

function type(fixture: ComponentFixture<WikiShellPage>, text: string): void {
    const input = fixture.nativeElement.querySelector(
        '[data-testid="wiki-search"]'
    ) as HTMLInputElement;
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
}

function find(fixture: ComponentFixture<WikiShellPage>, testId: string): Element | null {
    return (fixture.nativeElement as HTMLElement).querySelector(`[data-testid="${testId}"]`);
}

async function waitForDebounce(fixture: ComponentFixture<WikiShellPage>): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));
    fixture.detectChanges();
}

function stubOverlay(matches: boolean): void {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
        matches,
        addEventListener: () => undefined
    } as unknown as MediaQueryList);
}

beforeEach(() => stubOverlay(false));
afterEach(() => vi.restoreAllMocks());

describe('WikiShellPage search', () => {
    it('shows a loader instead of "nothing found" until the answer arrives', async () => {
        const responses = new Subject<WikiSearchHit[]>();
        const fixture = await render(responses);

        type(fixture, 'deploy');
        expect(find(fixture, 'wiki-search-loading')).not.toBeNull();
        expect(find(fixture, 'wiki-search-empty')).toBeNull();

        await waitForDebounce(fixture);
        expect(find(fixture, 'wiki-search-loading')).not.toBeNull();
        expect(find(fixture, 'wiki-search-empty')).toBeNull();

        responses.next([hit]);
        fixture.detectChanges();
        expect(find(fixture, 'wiki-search-loading')).toBeNull();
        expect(find(fixture, 'wiki-search-hit')?.textContent).toContain('Deploy');
    });

    it('says nothing was found only after an empty answer', async () => {
        const responses = new Subject<WikiSearchHit[]>();
        const fixture = await render(responses);

        type(fixture, 'nothing');
        await waitForDebounce(fixture);
        responses.next([]);
        fixture.detectChanges();

        expect(find(fixture, 'wiki-search-loading')).toBeNull();
        expect(find(fixture, 'wiki-search-empty')).not.toBeNull();
    });

    it('keeps the previous results visible while the next search runs', async () => {
        const responses = new Subject<WikiSearchHit[]>();
        const fixture = await render(responses);

        type(fixture, 'deploy');
        await waitForDebounce(fixture);
        responses.next([hit]);
        fixture.detectChanges();

        type(fixture, 'deploy steps');
        expect(find(fixture, 'wiki-search-loading')).not.toBeNull();
        expect(find(fixture, 'wiki-search-hit')).not.toBeNull();
    });
});

describe('WikiShellPage tree toggle', () => {
    afterEach(() => localStorage.removeItem('wiki.treeHidden'));

    it('hides the tree without leaving a strip behind and brings it back', async () => {
        localStorage.removeItem('wiki.treeHidden');
        const fixture = await render(new Subject<WikiSearchHit[]>());
        const element = fixture.nativeElement as HTMLElement;
        expect(find(fixture, 'wiki-nav')).not.toBeNull();

        element.querySelector<HTMLElement>('[data-testid="wiki-tree-hide"]')?.click();
        fixture.detectChanges();
        expect(find(fixture, 'wiki-nav')).toBeNull();
        expect(element.querySelector('.wiki-shell')?.children.length).toBe(1);

        TestBed.inject(WikiLayoutStore).toggleTree();
        fixture.detectChanges();
        expect(find(fixture, 'wiki-nav')).not.toBeNull();
    });
});

describe('WikiShellPage tree on narrow screens', () => {
    beforeEach(() => stubOverlay(true));

    it('lays the tree over the page instead of squeezing it and closes it on a tap outside', async () => {
        const fixture = await render(new Subject<WikiSearchHit[]>());
        const element = fixture.nativeElement as HTMLElement;
        const main = element.querySelector<HTMLElement>('.wiki-main')!;
        expect(find(fixture, 'wiki-nav')).toBeNull();
        const width = main.getBoundingClientRect().width;

        TestBed.inject(WikiLayoutStore).toggleTree();
        fixture.detectChanges();
        const nav = find(fixture, 'wiki-nav') as HTMLElement;
        expect(getComputedStyle(nav).position).toBe('absolute');
        expect(main.getBoundingClientRect().width).toBe(width);

        (find(fixture, 'wiki-nav-backdrop') as HTMLElement).click();
        fixture.detectChanges();
        expect(find(fixture, 'wiki-nav')).toBeNull();
    });

    it('closes the tree after navigating to a page', async () => {
        const fixture = await render(new Subject<WikiSearchHit[]>());
        TestBed.inject(WikiLayoutStore).toggleTree();
        fixture.detectChanges();
        expect(find(fixture, 'wiki-nav')).not.toBeNull();

        await TestBed.inject(Router).navigateByUrl('/project/1/wiki/project/deploy');
        fixture.detectChanges();
        expect(find(fixture, 'wiki-nav')).toBeNull();
    });
});
