import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { Component, viewChild } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { MessageEditorComponent } from './message-editor.component';
import { getLinearSelection, serialize, setLinearSelection } from './editor-text-model';
import { MessageModule } from '../../message.module';
import { User } from 'src/app/auth/model/user.model';

// Host so we can bind [message]/(messageChange) declaratively.
@Component({
    standalone: false,
    template: `
        <app-message-editor
            [mode]="mode"
            [change]="change"
            [message]="message"
            [mentionCandidates]="mentionCandidates"
            (messageChange)="onMessageChange($event)"
        ></app-message-editor>
    `
})
class HostComponent {
    public readonly editor = viewChild.required(MessageEditorComponent);
    public mode: 'edit' | 'create' = 'create';
    public change: 'onaction' | 'onchange' | 'onblur' = 'onaction';
    public message = '';
    public mentionCandidates: User[] = [];
    public emitted: string[] = [];
    public onMessageChange(v: string): void {
        this.emitted.push(v);
    }
}

function editorEl(fixture: ComponentFixture<HostComponent>): HTMLDivElement {
    return fixture.nativeElement.querySelector('.editor__content') as HTMLDivElement;
}

function placeCaretAtEnd(el: HTMLElement): void {
    el.focus();
    const range = document.createRange();
    range.selectNodeContents(el);
    range.collapse(false);
    const sel = window.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(range);
}

function selectAll(el: HTMLElement): void {
    el.focus();
    const range = document.createRange();
    range.selectNodeContents(el);
    const sel = window.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(range);
}

describe('MessageEditorComponent (contenteditable)', () => {
    let fixture: ComponentFixture<HostComponent>;
    let host: HostComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [
                MessageModule,
                ReactiveFormsModule,
                NoopAnimationsModule,
                TranslateModule.forRoot()
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(HostComponent);
        host = fixture.componentInstance;
        // Individual tests configure host props then call initFixture() so the
        // first change detection sees the final inputs (avoids NG0100 in the host).
    });

    async function initFixture(): Promise<void> {
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
    }

    afterEach(() => {
        fixture.destroy();
    });

    it('serializes typed text to the message model on send', async () => {
        await initFixture();
        const el = editorEl(fixture);
        el.textContent = 'hello world';
        placeCaretAtEnd(el);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();

        // Typing alone must NOT emit in onaction mode (would post per keystroke).
        expect(host.emitted).toEqual([]);

        // Shift+Enter = send.
        el.dispatchEvent(
            new KeyboardEvent('keydown', {
                key: 'Enter',
                shiftKey: true,
                bubbles: true
            })
        );
        fixture.detectChanges();
        await fixture.whenStable();

        expect(host.emitted).toContain('hello world');
        expect(host.editor().message()).toBe('hello world');
    });

    it('hydrates an existing body with a chip in edit mode and round-trips', async () => {
        host.mode = 'edit';
        host.message = 'hello @[Jan](user:1)';
        await initFixture();

        const el = editorEl(fixture);
        const chips = el.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(1);
        expect(chips[0].textContent).toBe('@Jan');
        expect(serialize(el)).toBe('hello @[Jan](user:1)');
    });

    it('Bold toolbar wraps the current selection in **; model/emit only on send (onaction)', async () => {
        await initFixture();
        const el = editorEl(fixture);
        el.textContent = 'abc';
        placeCaretAtEnd(el);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();

        selectAll(el);
        host.editor().onBold();
        fixture.detectChanges();
        await fixture.whenStable();

        // The toolbar edit wraps the selection in the DOM…
        expect(serialize(el)).toBe('**abc**');
        // …but in onaction mode a content edit must NOT emit messageChange (else a
        // comment editor would post to the server on every keystroke/toolbar click).
        expect(host.emitted).toEqual([]);

        // Shift+Enter (send) is the single commit that emits + updates the model.
        el.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true })
        );
        fixture.detectChanges();
        await fixture.whenStable();

        expect(host.emitted).toContain('**abc**');
        expect(host.editor().message()).toBe('**abc**');
    });

    it('list toolbar inserts the marker on the caret line, not the last text line, when the caret is on a trailing empty line', async () => {
        await initFixture();
        const el = editorEl(fixture);
        // "AAA" followed by an empty trailing line (browser's <div><br></div>).
        // serialize() trims the trailing newlines; the caret sits past that trimmed
        // length. The marker must still land on the empty line the caret is on.
        el.innerHTML = 'AAA<div><br></div>';
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        placeCaretAtEnd(el);

        host.editor().onUnorderedList();
        fixture.detectChanges();
        await fixture.whenStable();

        const out = serialize(el);
        // Bug behavior would prepend the marker onto the "AAA" line ("- AAA…").
        expect(out.startsWith('- ')).toBe(false);
        expect(out.split('\n')[0]).toBe('AAA');
        expect(out.split('\n').at(-1)).toBe('- ');
    });

    it('hydrate effect does not disturb caret when the same value is re-pushed', async () => {
        host.mode = 'edit';
        host.message = 'hello world';
        await initFixture();

        const el = editorEl(fixture);
        // Place caret at a mid-text position (offset 5 = between "hello" and " world").
        el.focus();
        const textNode = el.firstChild as Text;
        const midOffset = 5;
        const range = document.createRange();
        range.setStart(textNode, midOffset);
        range.collapse(true);
        const sel = window.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(range);

        // Push the SAME value back through the model input — this must not re-render.
        host.message = 'hello world';
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Caret must still be at linear offset 5.
        const { start, end } = getLinearSelection(el);
        expect(start).toBe(midOffset);
        expect(end).toBe(midOffset);
    });

    // Helper: make a minimal User fixture.
    function makeUser(idUser: number, name: string): User {
        return { idUser, name, email: '', colorAvatarBg: '#ccc' };
    }

    // Helper: place caret at a given linear offset inside the contenteditable.
    function placeCaretAtLinear(el: HTMLElement, offset: number): void {
        setLinearSelection(el, offset, offset);
    }

    it('typing "@ja" filters candidates and selecting via Enter inserts a chip token', async () => {
        host.mentionCandidates = [makeUser(1, 'Jan'), makeUser(2, 'Petra')];
        await initFixture();

        const el = editorEl(fixture);
        // Set editor content to "@ja" and place caret at end (linear offset 3).
        el.textContent = '@ja';
        placeCaretAtLinear(el, 3);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Overlay should show only "Jan" (matches "ja"), not "Petra".
        const items = fixture.nativeElement.querySelectorAll('.mention-picker__item');
        expect(items.length).toBe(1);
        expect(items[0].textContent).toContain('Jan');

        // Press Enter to select.
        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // DOM must contain a real .mention-chip element, NOT plain text "@[Jan](user:1)".
        const chips = el.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(1);
        expect(chips[0].textContent).toBe('@Jan');

        // Serialized editor content must be chip token + trailing space. In the
        // default 'onaction' mode a mention pick is an edit, not a send, so it does
        // NOT propagate to message()/messageChange — the token lives in the editor's
        // serialized DOM until the user actually sends.
        expect(serialize(el)).toBe('@[Jan](user:1) ');
    });

    it('Enter is NOT swallowed when @query has zero matching candidates (picker hidden)', async () => {
        host.mentionCandidates = [makeUser(1, 'Jan'), makeUser(2, 'Petra')];
        // Use onchange mode so typing propagates immediately and we can detect send separately.
        host.change = 'onaction';
        await initFixture();

        const el = editorEl(fixture);
        // Type "@zzz" — no candidates match "zzz".
        el.textContent = '@zzz';
        placeCaretAtLinear(el, 4);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Picker must be hidden (zero items).
        const items = fixture.nativeElement.querySelectorAll('.mention-picker__item');
        expect(items.length).toBe(0);

        // Shift+Enter must reach onSend (not be swallowed by picker logic).
        // Observable proof: _onChange is called (it triggers the registered CVA change
        // callback). We verify by checking that the dispatched event was NOT preventDefault'd,
        // because onShiftEnter calls evt.preventDefault() — so if it fired, it was prevented.
        const enterEvt = new KeyboardEvent('keydown', {
            key: 'Enter',
            shiftKey: true,
            bubbles: true,
            cancelable: true
        });
        el.dispatchEvent(enterEvt);
        fixture.detectChanges();
        await fixture.whenStable();

        // onShiftEnter calls evt.preventDefault(); if it was NOT swallowed by the picker
        // guard, the event reached that path and was prevented.
        expect(enterEvt.defaultPrevented).toBe(true);
    });

    it('plain Enter is NOT swallowed when @query has zero matching candidates', async () => {
        host.mentionCandidates = [makeUser(1, 'Jan')];
        await initFixture();

        const el = editorEl(fixture);
        // Type "@zzz" — no candidates match.
        el.textContent = '@zzz';
        placeCaretAtLinear(el, 4);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Picker must be hidden.
        const items = fixture.nativeElement.querySelectorAll('.mention-picker__item');
        expect(items.length).toBe(0);

        // Dispatch plain Enter with defaultPrevented tracking.
        const enterEvt = new KeyboardEvent('keydown', {
            key: 'Enter',
            bubbles: true,
            cancelable: true
        });
        el.dispatchEvent(enterEvt);
        fixture.detectChanges();

        // Plain Enter must NOT be prevented by picker logic when zero candidates.
        expect(enterEvt.defaultPrevented).toBe(false);
    });

    it('prefix safety: selecting "Janko" inserts user:2, not user:1', async () => {
        host.mentionCandidates = [makeUser(1, 'Jan'), makeUser(2, 'Janko')];
        await initFixture();

        const el = editorEl(fixture);
        // Type "@Janko" — both "Jan" and "Janko" match, but activeIndex will be 0 = "Jan" first.
        // We navigate down once to get to "Janko" (index 1).
        el.textContent = '@Janko';
        placeCaretAtLinear(el, 6);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Both match "janko" contains "jan", but only "Janko" contains "Janko" exactly.
        // Actually "Jan".includes("Janko") = false, "Janko".includes("Janko") = true.
        const items = fixture.nativeElement.querySelectorAll('.mention-picker__item');
        // Only "Janko" matches the query "Janko" (case-insensitive: "janko".includes("janko") = true,
        // "jan".includes("janko") = false).
        expect(items.length).toBe(1);
        expect(items[0].textContent).toContain('Janko');

        // Select via Enter (activeIndex = 0 = Janko since it's the only one).
        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // Serialized editor content carries the picked user's token (a mention pick
        // does not propagate to message() in the default 'onaction' mode).
        const msg = serialize(el);
        expect(msg).toContain('user:2');
        expect(msg).not.toContain('user:1');
    });

    it('@-picker does NOT open when @ follows a literal ) in plain text', async () => {
        // "foo)@jan" — the ')' immediately precedes '@' but is plain text, not a chip.
        // atMentionBoundary() must reject this position so the picker stays closed.
        host.mentionCandidates = [makeUser(1, 'Jan')];
        await initFixture();

        const el = editorEl(fixture);
        // Insert "foo)@ja" as a single text node so the '@' is preceded by ')'.
        el.textContent = 'foo)@ja';
        placeCaretAtLinear(el, 7);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // The picker must NOT be visible — zero items.
        const items = fixture.nativeElement.querySelectorAll('.mention-picker__item');
        expect(items.length).toBe(0);
    });

    it('plain Enter inserts a newline (does not send); Shift+Enter sends', async () => {
        await initFixture();
        const el = editorEl(fixture);
        el.textContent = 'line1';
        placeCaretAtEnd(el);
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();

        const before = host.emitted.length;
        // Plain Enter must NOT send.
        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        expect(host.emitted.length).toBe(before);

        // Simulate the browser-inserted newline + typed text and serialize.
        el.appendChild(document.createElement('br'));
        el.appendChild(document.createTextNode('line2'));
        el.dispatchEvent(new Event('input', { bubbles: true }));
        fixture.detectChanges();
        expect(serialize(el)).toBe('line1\nline2');

        // Shift+Enter sends the multi-line value.
        el.dispatchEvent(
            new KeyboardEvent('keydown', {
                key: 'Enter',
                shiftKey: true,
                bubbles: true
            })
        );
        fixture.detectChanges();
        await fixture.whenStable();
        expect(host.emitted).toContain('line1\nline2');
    });
});
