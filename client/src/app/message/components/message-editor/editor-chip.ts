import { IconLoader2, IconPaperclip } from '@tabler/icons-angular';
import { Mention } from 'src/app/shared/mention/mention';
import { UiTablerSvg } from 'src/app/ui/util/ui-tabler-svg';

const ATTACHMENT_CHIP_CLASS =
    'attachment-chip inline-flex max-w-64 select-none items-center gap-1 whitespace-nowrap rounded-[var(--ui-radius-sm)] ui-bg-surface-100 px-[0.35em] align-bottom';

export abstract class EditorChip {
    public static isChip(n: Node): n is HTMLElement {
        const classList = n.nodeType === 1 ? (n as HTMLElement).classList : undefined;
        return (
            !!classList &&
            (classList.contains('mention-chip') || classList.contains('attachment-chip'))
        );
    }

    public static buildAttachmentChip(markdown: string, name: string): HTMLElement {
        const s = document.createElement('span');
        s.className = `${ATTACHMENT_CHIP_CLASS} ui-color-text`;
        s.contentEditable = 'false';
        s.dataset['token'] = markdown;
        s.append(UiTablerSvg.create(IconPaperclip, 14, 'shrink-0'), EditorChip.chipName(name));
        return s;
    }

    public static buildUploadPlaceholder(name: string): HTMLElement {
        const s = document.createElement('span');
        s.className = `${ATTACHMENT_CHIP_CLASS} attachment-chip--uploading ui-color-text-muted`;
        s.contentEditable = 'false';
        s.dataset['token'] = '';
        s.append(
            UiTablerSvg.create(IconLoader2, 14, 'shrink-0 animate-spin'),
            EditorChip.chipName(name)
        );
        return s;
    }

    private static chipName(name: string): HTMLElement {
        const label = document.createElement('span');
        label.className = 'truncate';
        label.textContent = name;
        return label;
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
        EditorChip.insertNodeAtRange(
            root,
            sel,
            sel.getRangeAt(0),
            EditorChip.buildChip(idUser, name)
        );
    }

    public static insertNodeInEditor(root: HTMLElement, node: HTMLElement): void {
        const sel = root.ownerDocument.getSelection();
        if (!sel) return;
        let r = sel.rangeCount ? sel.getRangeAt(0) : null;
        if (!r || !root.contains(r.commonAncestorContainer)) {
            r = root.ownerDocument.createRange();
            r.selectNodeContents(root);
            r.collapse(false);
        }
        EditorChip.insertNodeAtRange(root, sel, r, node);
    }

    private static insertNodeAtRange(
        root: HTMLElement,
        sel: Selection,
        r: Range,
        chip: HTMLElement
    ): void {
        r.deleteContents();
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
