import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { Issue } from 'src/app/issue/model/issue.model';
import { WikiLinkScope } from 'src/app/wiki/service/wiki-link-scope.service';
import { MessageModule } from '../../message.module';
import { EditorText } from './editor-text';

@Component({
    standalone: false,
    template: '<app-message-editor [idProject]="idProject"></app-message-editor>'
})
class HostComponent {
    public idProject: number | null = null;
}

const issue = (idIssuePublic: number, title: string): Issue => {
    return {
        idIssue: idIssuePublic,
        idIssuePublic,
        idProject: 7,
        idState: null,
        idSeverity: null,
        idIssueType: null,
        title,
        description: '',
        tracked: 0
    };
};

const issues: Issue[] = [issue(123, 'Fix login'), issue(456, 'Add dark mode')];

async function render(
    idProject: number | null,
    issues$: Subject<Issue[]>,
    hostIdProject: number | null = null
): Promise<{ fixture: ComponentFixture<HostComponent>; load: ReturnType<typeof vi.fn> }> {
    const load = vi.fn().mockReturnValue(issues$);
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [MessageModule, NoopAnimationsModule, TranslateModule.forRoot()],
        providers: [{ provide: IssueApi, useValue: { load$: load } }, WikiLinkScope]
    }).compileComponents();
    TestBed.inject(WikiLinkScope).idProject.set(idProject);
    const fixture = TestBed.createComponent(HostComponent);
    fixture.componentInstance.idProject = hostIdProject;
    fixture.detectChanges();
    await fixture.whenStable();
    return { fixture, load };
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
        (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="issue-link-option"]')
    ).map(option => option.textContent?.trim() ?? '');
}

function picker(fixture: ComponentFixture<HostComponent>): HTMLElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '[data-testid="issue-link-picker"]'
    );
}

describe('MessageEditorComponent issue links', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('opens the task picker after #, loads the project issues and filters by the typed number', async () => {
        const issues$ = new Subject<Issue[]>();
        const { fixture, load } = await render(7, issues$);

        type(fixture, 'see #12');
        expect(load).toHaveBeenCalledWith({
            idProject: 7,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(picker(fixture)).not.toBeNull();
        expect(options(fixture)).toEqual([]);

        issues$.next(issues);
        fixture.detectChanges();
        expect(options(fixture)).toEqual(['#123Fix login']);

        type(fixture, 'see #45');
        expect(options(fixture)).toEqual(['#456Add dark mode']);
    });

    it('inserts the picked issue as a #reference with a trailing space', async () => {
        const issues$ = new Subject<Issue[]>();
        const { fixture } = await render(7, issues$);
        const editor = type(fixture, 'see #45');
        issues$.next(issues);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see #456 ');
        expect(picker(fixture)).toBeNull();
    });

    it('loads the issue list once per project across picker opens', async () => {
        const issues$ = new Subject<Issue[]>();
        const { fixture, load } = await render(7, issues$);

        type(fixture, 'see #12');
        issues$.next(issues);
        fixture.detectChanges();
        type(fixture, 'see');
        type(fixture, 'see #12');
        expect(load).toHaveBeenCalledTimes(1);
    });

    it('stays a plain editor outside a project', async () => {
        const issues$ = new Subject<Issue[]>();
        const { fixture, load } = await render(null, issues$);

        type(fixture, 'see #12');
        expect(load).not.toHaveBeenCalled();
        expect(picker(fixture)).toBeNull();
    });

    it('uses the idProject input when no wiki scope is provided (project chat)', async () => {
        const issues$ = new Subject<Issue[]>();
        const { fixture, load } = await render(null, issues$, 3);

        type(fixture, 'see #1');
        expect(load).toHaveBeenCalledWith({
            idProject: 3,
            orderColumn: 'createAt',
            orderDirection: 'desc'
        });
        expect(picker(fixture)).not.toBeNull();
    });
});
