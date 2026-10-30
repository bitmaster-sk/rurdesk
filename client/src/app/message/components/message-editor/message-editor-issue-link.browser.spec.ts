import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { Subject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IssueLinkApi } from 'src/app/link/api/issue-link.api.service';
import { IssueLinkOption } from 'src/app/link/model/issue-link.model';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { MessageModule } from '../../message.module';
import { EditorText } from './editor-text';

@Component({
    standalone: false,
    template: '<app-message-editor [message]="message"></app-message-editor>'
})
class HostComponent {
    public message = '';
}

const options: IssueLinkOption[] = [
    { idProject: 7, projectName: 'Rurdesk', idIssuePublic: 123, title: 'Fix login' },
    { idProject: 9, projectName: 'Website', idIssuePublic: 12, title: 'Blog redesign' }
];

async function render(opts: { idProject?: number | null; message?: string } = {}): Promise<{
    fixture: ComponentFixture<HostComponent>;
    load: ReturnType<typeof vi.fn>;
}> {
    const load = vi.fn().mockImplementation(() => new Subject<IssueLinkOption[]>());
    const providers: object[] = [{ provide: IssueLinkApi, useValue: { load$: load } }];
    if (opts.idProject !== undefined) {
        providers.push(LinkScope);
    }
    await TestBed.configureTestingModule({
        declarations: [HostComponent],
        imports: [MessageModule, NoopAnimationsModule, TranslateModule.forRoot()],
        providers
    }).compileComponents();
    if (opts.idProject !== undefined) {
        TestBed.inject(LinkScope).idProject.set(opts.idProject);
    }
    const fixture = TestBed.createComponent(HostComponent);
    fixture.componentInstance.message = opts.message ?? '';
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, load };
}

function editorOf(fixture: ComponentFixture<HostComponent>): HTMLDivElement {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '.editor__content'
    ) as HTMLDivElement;
}

function placeCaretAtEnd(editor: HTMLDivElement): void {
    editor.focus();
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
}

async function type(
    fixture: ComponentFixture<HostComponent>,
    text: string
): Promise<HTMLDivElement> {
    const editor = editorOf(fixture);
    editor.textContent = text;
    placeCaretAtEnd(editor);
    editor.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    await new Promise(resolve => setTimeout(resolve, 200));
    fixture.detectChanges();
    return editor;
}

function optionTexts(fixture: ComponentFixture<HostComponent>): string[] {
    return Array.from(
        (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="issue-link-option"]')
    ).map(option => option.textContent?.replace(/\s+/g, ' ').trim() ?? '');
}

function picker(fixture: ComponentFixture<HostComponent>): HTMLElement | null {
    return (fixture.nativeElement as HTMLElement).querySelector(
        '[data-testid="issue-link-picker"]'
    );
}

describe('MessageEditorComponent issue links', () => {
    afterEach(() => TestBed.resetTestingModule());

    it('opens the task picker after # in a chat without a project and lists tasks with their project', async () => {
        const { fixture, load } = await render();

        await type(fixture, 'see #fix');
        expect(load).toHaveBeenCalledWith('fix', null);
        expect(picker(fixture)).not.toBeNull();
        expect(optionTexts(fixture)).toEqual([]);

        load.mock.results[0].value.next(options);
        fixture.detectChanges();

        expect(optionTexts(fixture)).toEqual([
            'Rurdesk #123 Fix login',
            'Website #12 Blog redesign'
        ]);
    });

    it('asks for the tasks of the current project first', async () => {
        const { fixture, load } = await render({ idProject: 7 });

        await type(fixture, 'see #12');

        expect(load).toHaveBeenCalledWith('12', 7);
    });

    it('does not offer the tasks of the previous search while the next one is running', async () => {
        const { fixture, load } = await render();
        await type(fixture, 'see #1');
        load.mock.results[0].value.next(options);
        fixture.detectChanges();

        const editor = await type(fixture, 'see #12');
        expect(load).toHaveBeenCalledTimes(2);
        expect(optionTexts(fixture)).toEqual([]);
        expect(picker(fixture)).not.toBeNull();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        expect(editor.querySelector('.issue-link-chip')).toBeNull();

        load.mock.results[1].value.next([options[1]]);
        fixture.detectChanges();
        expect(optionTexts(fixture)).toEqual(['Website #12 Blog redesign']);
    });

    it('picks the first task of a narrowed search even after moving down the longer list', async () => {
        const { fixture, load } = await render();
        const editor = await type(fixture, 'see #1');
        load.mock.results[0].value.next(options);
        fixture.detectChanges();
        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));

        await type(fixture, 'see #12');
        load.mock.results[1].value.next([options[1]]);
        fixture.detectChanges();
        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see [#12](/project/9/issue/12) ');
    });

    it('inserts the picked task as a #number chip stored as a full link', async () => {
        const { fixture, load } = await render();
        const editor = await type(fixture, 'see #12');
        load.mock.results[0].value.next(options);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();

        expect(EditorText.serialize(editor)).toBe('see [#12](/project/9/issue/12) ');
        expect(editor.querySelector('.issue-link-chip')?.textContent).toBe('#12');
        expect(picker(fixture)).toBeNull();
    });

    it('highlights one option, keeps the keyboard choice under a still mouse and follows real mouse moves', async () => {
        const { fixture, load } = await render();
        const editor = await type(fixture, 'see #1');
        load.mock.results[0].value.next(options);
        fixture.detectChanges();
        const active = (): string[] =>
            Array.from(
                (fixture.nativeElement as HTMLElement).querySelectorAll(
                    '.issue-link-picker__item--active'
                )
            ).map(option => option.textContent?.replace(/\s+/g, ' ').trim() ?? '');
        expect(active()).toEqual(['Rurdesk #123 Fix login']);

        (fixture.nativeElement as HTMLElement)
            .querySelectorAll('[data-testid="issue-link-option"]')[1]
            .dispatchEvent(new PointerEvent('pointermove', { bubbles: true }));
        fixture.detectChanges();
        expect(active()).toEqual(['Rurdesk #123 Fix login']);

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        fixture.detectChanges();
        expect(active()).toEqual(['Website #12 Blog redesign']);

        (fixture.nativeElement as HTMLElement)
            .querySelectorAll('[data-testid="issue-link-option"]')[0]
            .dispatchEvent(new PointerEvent('pointermove', { bubbles: true, movementY: 3 }));
        fixture.detectChanges();
        expect(active()).toEqual(['Rurdesk #123 Fix login']);

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        expect(EditorText.serialize(editor)).toBe('see [#123](/project/7/issue/123) ');
    });

    it('closes the picker on Escape without inserting anything', async () => {
        const { fixture, load } = await render();
        const editor = await type(fixture, 'see #12');
        load.mock.results[0].value.next(options);
        fixture.detectChanges();

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        fixture.detectChanges();

        expect(picker(fixture)).toBeNull();
        expect(EditorText.serialize(editor)).toBe('see #12');
    });

    it('shows a stored task link as a chip when an existing message is edited', async () => {
        const { fixture } = await render({
            message: 'blocked by [#123](/project/7/issue/123) today'
        });
        const editor = editorOf(fixture);

        expect(editor.querySelector('.issue-link-chip')?.textContent).toBe('#123');
        expect(EditorText.serialize(editor)).toBe('blocked by [#123](/project/7/issue/123) today');
    });

    it('shows a task link as a highlighted chip', async () => {
        const { fixture } = await render({ message: 'see [#123](/project/7/issue/123)' });
        const chip = editorOf(fixture).querySelector<HTMLElement>('.issue-link-chip')!;

        expect(getComputedStyle(chip).display).toBe('inline-block');
    });

    it('removes the whole task link with one Backspace', async () => {
        const { fixture } = await render({ message: 'see [#123](/project/7/issue/123)' });
        const editor = editorOf(fixture);
        placeCaretAtEnd(editor);

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true }));
        fixture.detectChanges();

        expect(editor.querySelector('.issue-link-chip')).toBeNull();
        expect(EditorText.serialize(editor)).toBe('see ');
    });

    it('removes the whole task link with one Delete in front of it', async () => {
        const { fixture } = await render({ message: '[#123](/project/7/issue/123) done' });
        const editor = editorOf(fixture);
        editor.focus();
        const range = document.createRange();
        range.setStart(editor, 0);
        range.collapse(true);
        window.getSelection()!.removeAllRanges();
        window.getSelection()!.addRange(range);

        editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Delete', bubbles: true }));
        fixture.detectChanges();

        expect(editor.querySelector('.issue-link-chip')).toBeNull();
        expect(EditorText.serialize(editor)).toBe(' done');
    });
});
