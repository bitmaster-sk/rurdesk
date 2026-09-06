import { Mention } from 'src/app/shared/mention/mention';

export abstract class EditorChip {
    public static isChip(n: Node): n is HTMLElement {
        return n.nodeType === 1 && (n as HTMLElement).classList?.contains('mention-chip');
    }

    // Build an atomic chip element for insertion.
    public static buildChip(idUser: number, name: string): HTMLElement {
        const s = document.createElement('span');
        s.className = 'mention-chip';
        s.contentEditable = 'false';
        s.dataset['token'] = Mention.serialize(idUser, name);
        s.dataset['id'] = String(idUser);
        s.textContent = '@' + name;
        return s;
    }

    // True when the caret is at a valid '@' mention opener:
    // preceding char is whitespace/BOF, or the immediately preceding DOM node is a chip.
    public static atMentionBoundary(root: HTMLElement): boolean {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount) return false;
        const r = sel.getRangeAt(0);
        const c = r.startContainer;
        const o = r.startOffset;
        if (c.nodeType === 3) {
            if (o === 0) {
                const prev = c.previousSibling;
                return !prev || EditorChip.isChip(prev);
            }
            return /\s/.test((c as Text).data[o - 1]);
        }
        const prev = c.childNodes[o - 1];
        return (
            !prev ||
            EditorChip.isChip(prev) ||
            (prev.nodeType === 3 && /\s$/.test((prev as Text).data))
        );
    }

    // Insert a chip at the caret position, then place the caret after the trailing space.
    // Range.insertNode (not execCommand) so the chip stays one atomic node.
    public static insertChipAtCaret(root: HTMLElement, idUser: number, name: string): void {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount) return;
        const r = sel.getRangeAt(0);
        r.deleteContents();
        const chip = EditorChip.buildChip(idUser, name);
        const space = root.ownerDocument.createTextNode(' ');
        r.insertNode(space);
        r.insertNode(chip);
        const nr = root.ownerDocument.createRange();
        nr.setStartAfter(space);
        nr.collapse(true);
        sel.removeAllRanges();
        sel.addRange(nr);
    }
}
