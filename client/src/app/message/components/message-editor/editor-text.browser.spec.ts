import { describe, it, expect, beforeEach, afterEach } from 'vitest';
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

describe('EditorText.serialize', () => {
    let root: HTMLElement;

    beforeEach(() => {
        root = makeRoot();
    });

    afterEach(() => {
        root.remove();
    });

    it('serializes plain text', () => {
        root.textContent = 'hello';
        expect(EditorText.serialize(root)).toBe('hello');
    });

    it('serializes a chip to its token', () => {
        root.innerHTML = `cc ${chip('@[Jan](user:1)', '@Jan')} ok`;
        expect(EditorText.serialize(root)).toBe('cc @[Jan](user:1) ok');
    });

    it('serializes text+chip with correct spacing', () => {
        root.innerHTML = `a${chip('@[Jan](user:1)', '@Jan')}b`;
        expect(EditorText.serialize(root)).toBe('a@[Jan](user:1)b');
    });

    it('serializes a <br> as \\n', () => {
        // two text nodes separated by a <br>
        const t1 = document.createTextNode('line1');
        const br = document.createElement('br');
        const t2 = document.createTextNode('line2');
        root.append(t1, br, t2);
        expect(EditorText.serialize(root)).toBe('line1\nline2');
    });

    it('serializes a block <div> as \\n separator', () => {
        // Chrome/WebKit model for Enter: first line in root, second in a <div>
        root.innerHTML = 'line1<div>line2</div>';
        expect(EditorText.serialize(root)).toBe('line1\nline2');
    });

    it('trims trailing newlines from browser-appended <div><br></div>', () => {
        root.innerHTML = 'text<div><br></div>';
        expect(EditorText.serialize(root)).toBe('text');
    });

    it('keeps the trailing newlines serialize() trims', () => {
        root.innerHTML = 'text<div><br></div>';
        expect(EditorText.serializeRaw(root).startsWith('text\n')).toBe(true);
    });
});
