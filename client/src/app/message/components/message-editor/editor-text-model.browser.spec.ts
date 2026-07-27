import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
    serialize,
    getLinearSelection,
    setLinearSelection,
    atMentionBoundary,
    buildChip
} from './editor-text-model';

// ---- helpers ----

function makeRoot(): HTMLElement {
    const div = document.createElement('div');
    div.contentEditable = 'true';
    document.body.appendChild(div);
    return div;
}

function chip(token: string, label: string): string {
    return `<span class="mention-chip" contenteditable="false" data-token="${token}" data-id="1">${label}</span>`;
}

// ---- suite ----

describe('editor-text-model', () => {
    let root: HTMLElement;

    beforeEach(() => {
        root = makeRoot();
    });

    afterEach(() => {
        root.remove();
    });

    // --- serialize ---

    it('serializes plain text', () => {
        root.textContent = 'hello';
        expect(serialize(root)).toBe('hello');
    });

    it('serializes a chip to its token', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        expect(serialize(root)).toBe('cc @[Jan](user:1) ok');
    });

    it('serializes text+chip with correct spacing', () => {
        root.innerHTML = `a${chip('@[Jan](user:1)', '@Jan')}b`;
        expect(serialize(root)).toBe('a@[Jan](user:1)b');
    });

    it('serializes a <br> as \\n', () => {
        // two text nodes separated by a <br>
        const t1 = document.createTextNode('line1');
        const br = document.createElement('br');
        const t2 = document.createTextNode('line2');
        root.append(t1, br, t2);
        expect(serialize(root)).toBe('line1\nline2');
    });

    it('serializes a block <div> as \\n separator', () => {
        // Chrome/WebKit model for Enter: first line in root, second in a <div>
        root.innerHTML = 'line1<div>line2</div>';
        expect(serialize(root)).toBe('line1\nline2');
    });

    it('trims trailing newlines from browser-appended <div><br></div>', () => {
        root.innerHTML = 'text<div><br></div>';
        expect(serialize(root)).toBe('text');
    });

    // --- caret round-trip ---

    it('round-trips caret at start of text before chip', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        // offset 0: before "cc "
        setLinearSelection(root, 0, 0);
        expect(getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });

    it('round-trips caret at offset 2 (inside "cc")', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        setLinearSelection(root, 2, 2);
        expect(getLinearSelection(root)).toEqual({ start: 2, end: 2 });
    });

    it('round-trips caret at end of string', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const s = serialize(root); // "cc @[Jan](user:1) ok"
        setLinearSelection(root, s.length, s.length);
        expect(getLinearSelection(root)).toEqual({ start: s.length, end: s.length });
    });

    it('round-trips caret at chipStart (before chip)', () => {
        // "cc " = 3 chars, then chip "@[Jan](user:1)" = 14 chars
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const chipStart = 3; // offset right before chip token
        setLinearSelection(root, chipStart, chipStart);
        const result = getLinearSelection(root);
        expect(result.start).toBe(chipStart);
        expect(result.end).toBe(chipStart);
    });

    it('round-trips caret at chipEnd (after chip)', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const chipEnd = 3 + '@[Jan](user:1)'.length; // 17
        setLinearSelection(root, chipEnd, chipEnd);
        const result = getLinearSelection(root);
        expect(result.start).toBe(chipEnd);
        expect(result.end).toBe(chipEnd);
    });

    it('interior-of-chip offsets snap to a chip boundary', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const chipStart = 3;
        const chipLen = '@[Jan](user:1)'.length; // 14
        const chipEnd = chipStart + chipLen;
        // Try each interior offset (chipStart+1 .. chipEnd-1) and assert the result
        // snaps to either chipStart or chipEnd
        for (let t = chipStart + 1; t < chipEnd; t++) {
            setLinearSelection(root, t, t);
            const result = getLinearSelection(root);
            expect(result.start === chipStart || result.start === chipEnd).toBe(true);
            expect(result.end === chipStart || result.end === chipEnd).toBe(true);
        }
    });

    // --- empty / root caret ---

    it('returns {0,0} for an empty root', () => {
        setLinearSelection(root, 0, 0);
        expect(getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });

    it('returns {0,0} when selection is absent', () => {
        // no setLinearSelection call; root is empty
        // getSelection() on a fresh element has no range
        const sel = document.getSelection();
        sel?.removeAllRanges();
        expect(getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });

    // --- atMentionBoundary ---

    it('atMentionBoundary: true right after a chip', () => {
        root.innerHTML = `${chip('@[Jan](user:1)', '@Jan')} `;
        // place caret in the text node after the chip, at position 0
        const textNode = root.lastChild as Text;
        const r = document.createRange();
        r.setStart(textNode, 0);
        r.collapse(true);
        const sel = document.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(r);
        expect(atMentionBoundary(root)).toBe(true);
    });

    it('atMentionBoundary: false mid-text', () => {
        root.innerHTML = 'hello world';
        const textNode = root.firstChild as Text;
        const r = document.createRange();
        r.setStart(textNode, 3); // 'l' in 'hello'
        r.collapse(true);
        const sel = document.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(r);
        expect(atMentionBoundary(root)).toBe(false);
    });

    it('atMentionBoundary: true after whitespace', () => {
        root.innerHTML = 'hello ';
        const textNode = root.firstChild as Text;
        const r = document.createRange();
        r.setStart(textNode, 6); // after the space
        r.collapse(true);
        const sel = document.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(r);
        expect(atMentionBoundary(root)).toBe(true);
    });

    it('atMentionBoundary: false when no selection', () => {
        const sel = document.getSelection();
        sel?.removeAllRanges();
        expect(atMentionBoundary(root)).toBe(false);
    });

    // --- buildChip ---

    it('buildChip produces correct token and label', () => {
        const el = buildChip(42, 'Alice');
        expect(el.dataset['token']).toBe('@[Alice](user:42)');
        expect(el.dataset['id']).toBe('42');
        expect(el.textContent).toBe('@Alice');
        expect(el.className).toBe('mention-chip');
        expect(el.contentEditable).toBe('false');
    });
});
