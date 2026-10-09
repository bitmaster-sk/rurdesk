import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { WikiAgentAccess } from 'src/app/wiki/constants/wiki-agent-access.enum';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { WikiTree, WikiTreeNode } from 'src/app/wiki/model/wiki-tree.model';
import { WikiLinkScope } from 'src/app/wiki/service/wiki-link-scope.service';
import { MessageModule } from '../../message.module';
import { EditorText } from './editor-text';

@Component({
    standalone: false,
    template: '<app-message-editor></app-message-editor>'
})
class HostComponent {}

const node = (idPage: number, idSpace: number, slug: string, title: string): WikiTreeNode => ({
    idPage,
    idSpace,
    idParent: null,
    slug,
    title,
    agentAccess: WikiAgentAccess.OnDemand,
    rank: 'm'
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
        node(10, 2, 'deploy-runbook', 'Deploy runbook'),
        node(11, 1, 'deploy-policy', 'Deploy policy'),
        node(12, 2, 'release', 'Release')
    ],
    alwaysTokens: 0,
    tokenLimit: 0,
    trashCount: 0,
    proposalCount: 0
};

async function render(
    idProject: number | null,
    trees: Subject<WikiTree>
): Promise<{ fixture: ComponentFixture<HostComponent>; loadTree: ReturnType<typeof vi.fn> }> {
    const loadTree = vi.fn().mockReturnValue(trees);
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [MessageModule, NoopAnimationsModule, TranslateModule.forRoot()],
        providers: [{ provide: WikiApi, useValue: { loadTree$: loadTree } }, WikiLinkScope]
    }).compileComponents();
    TestBed.inject(WikiLinkScope).idProject.set(idProject);
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    return { fixture, loadTree };
}

function type(fixture: ComponentFixture<HostComponent>, text: string): HTMLDivElement {
    const editor = fixture.nativeElement.querySelector('.editor__content') as HTMLDivElement;
    editor.focus();
    editor.textContent = text;
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    editor.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    return editor;
}

function options(fixture: ComponentFixture<HostComponent>): string[] {
    return Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="wiki-link-option"]')
    ).map(option => option.textContent?.trim() ?? '');
}

describe('MessageEditorComponent wiki links', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('opens a page picker after [[ that loads the wiki and filters by the typed text', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture, loadTree } = await render(7, trees);

        type(fixture, 'see [[depl');
        expect(loadTree).toHaveBeenCalledWith(7);
        expect(
            (fixture.nativeElement as HTMLElement).querySelector('[data-testid="wiki-link-picker"]')
        ).not.toBeNull();
        expect(options(fixture)).toEqual([]);

        trees.next(tree);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['Deploy runbook', 'Deploy policy WIKI.SPACE.INSTANCE']);
    });

    it('inserts the picked page as a wiki link, with the shared prefix for shared pages', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture } = await render(7, trees);
        const editor = type(fixture, 'see [[depl');
        trees.next(tree);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see [[shared:Deploy policy]]');
        expect(options(fixture)).toEqual([]);
    });

    it('stays a plain editor outside a project', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture, loadTree } = await render(null, trees);

        type(fixture, '[[depl');
        expect(loadTree).not.toHaveBeenCalled();
        expect(
            (fixture.nativeElement as HTMLElement).querySelector('[data-testid="wiki-link-picker"]')
        ).toBeNull();
    });

    it('opens the picker at the typed [[ rather than at the edge of the editor', async () => {
        const trees = new Subject<WikiTree>();
        const { fixture } = await render(7, trees);
        trees.next(tree);
        const picker = (): HTMLElement =>
            (fixture.nativeElement as HTMLElement).querySelector(
                '[data-testid="wiki-link-picker"]'
            ) as HTMLElement;

        type(fixture, '[[depl');
        const atStart = picker().getBoundingClientRect().left;
        type(fixture, 'a few words first [[depl');
        const later = picker().getBoundingClientRect().left;

        expect(later).toBeGreaterThan(atStart + 50);
    });
});
