import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { EditorChip } from './editor-chip';

function makeRoot(): HTMLElement {
    const div = document.createElement('div');
    div.contentEditable = 'true';
    document.body.appendChild(div);
    return div;
}

function chip(token: string, label: string): string {
    return `<span class="mention-chip" contenteditable="false" data-token="${token}" data-id="1">${label}</span>`;
}

function caretAt(node: Node, offset: number): void {
    const r = document.createRange();
    r.setStart(node, offset);
    r.collapse(true);
    const sel = document.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(r);
}

describe('EditorChip', () => {
    let root: HTMLElement;

    beforeEach(() => {
        root = makeRoot();
    });

    afterEach(() => {
        root.remove();
    });

    it('atMentionBoundary: true right after a chip', () => {
        root.innerHTML = `${chip('@[Jan](user:1)', '@Jan')} `;
        caretAt(root.lastChild as Text, 0);
        expect(EditorChip.atMentionBoundary(root)).toBe(true);
    });

    it('atMentionBoundary: false mid-text', () => {
        root.innerHTML = 'hello world';
        caretAt(root.firstChild as Text, 3); // 'l' in 'hello'
        expect(EditorChip.atMentionBoundary(root)).toBe(false);
    });

    it('atMentionBoundary: true after whitespace', () => {
        root.innerHTML = 'hello ';
        caretAt(root.firstChild as Text, 6); // after the space
        expect(EditorChip.atMentionBoundary(root)).toBe(true);
    });

    it('atMentionBoundary: false when no selection', () => {
        const sel = document.getSelection();
        sel?.removeAllRanges();
        expect(EditorChip.atMentionBoundary(root)).toBe(false);
    });

    it('buildChip produces correct token and label', () => {
        const el = EditorChip.buildChip(42, 'Alice');
        expect(el.dataset['token']).toBe('@[Alice](user:42)');
        expect(el.dataset['id']).toBe('42');
        expect(el.textContent).toBe('@Alice');
        expect(el.className).toBe('mention-chip');
        expect(el.contentEditable).toBe('false');
    });

    it('insertChipAtCaret puts the chip and a trailing space at the caret', () => {
        root.textContent = 'hi ';
        caretAt(root.firstChild as Text, 3);
        EditorChip.insertChipAtCaret(root, 7, 'Bob');
        const inserted = root.querySelector('.mention-chip') as HTMLElement;
        expect(inserted.dataset['token']).toBe('@[Bob](user:7)');
        expect(root.textContent).toBe('hi @Bob ');
    });
});
