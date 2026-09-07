import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { EditorSelection } from './editor-selection';
import { EditorText } from './editor-text';

function makeRoot(): HTMLElement {
    const div = document.createElement('div');
    div.contentEditable = 'true';
    document.body.appendChild(div);
    return div;
}

function chip(token: string, label: string): string {
    return `<span class="mention-chip" contenteditable="false" data-token="${token}" data-id="1">${label}</span>`;
}

describe('EditorSelection', () => {
    let root: HTMLElement;

    beforeEach(() => {
        root = makeRoot();
    });

    afterEach(() => {
        root.remove();
    });

    it('round-trips caret at start of text before chip', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        // offset 0: before "cc "
        EditorSelection.setLinearSelection(root, 0, 0);
        expect(EditorSelection.getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });

    it('round-trips caret at offset 2 (inside "cc")', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        EditorSelection.setLinearSelection(root, 2, 2);
        expect(EditorSelection.getLinearSelection(root)).toEqual({ start: 2, end: 2 });
    });

    it('round-trips caret at end of string', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const s = EditorText.serialize(root); // "cc @[Jan](user:1) ok"
        EditorSelection.setLinearSelection(root, s.length, s.length);
        expect(EditorSelection.getLinearSelection(root)).toEqual({
            start: s.length,
            end: s.length
        });
    });

    it('round-trips caret at chipStart (before chip)', () => {
        // "cc " = 3 chars, then chip "@[Jan](user:1)" = 14 chars
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const chipStart = 3; // offset right before chip token
        EditorSelection.setLinearSelection(root, chipStart, chipStart);
        const result = EditorSelection.getLinearSelection(root);
        expect(result.start).toBe(chipStart);
        expect(result.end).toBe(chipStart);
    });

    it('round-trips caret at chipEnd (after chip)', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        const chipEnd = 3 + '@[Jan](user:1)'.length; // 17
        EditorSelection.setLinearSelection(root, chipEnd, chipEnd);
        const result = EditorSelection.getLinearSelection(root);
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
            EditorSelection.setLinearSelection(root, t, t);
            const result = EditorSelection.getLinearSelection(root);
            expect(result.start === chipStart || result.start === chipEnd).toBe(true);
            expect(result.end === chipStart || result.end === chipEnd).toBe(true);
        }
    });

    it('returns {0,0} for an empty root', () => {
        EditorSelection.setLinearSelection(root, 0, 0);
        expect(EditorSelection.getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });

    it('returns {0,0} when selection is absent', () => {
        // no setLinearSelection call; root is empty
        // getSelection() on a fresh element has no range
        const sel = document.getSelection();
        sel?.removeAllRanges();
        expect(EditorSelection.getLinearSelection(root)).toEqual({ start: 0, end: 0 });
    });
});
