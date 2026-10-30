import { IconLoader2, IconPaperclip } from '@tabler/icons-angular';
import { IssueLinkConverter } from 'src/app/link/converter/issue-link.converter';
import { IssueLinkRef } from 'src/app/link/model/issue-link.model';
import { Mention } from 'src/app/shared/mention/mention';
import { UiTablerSvg } from 'src/app/ui/util/ui-tabler-svg';

const ATTACHMENT_CHIP_CLASS =
    'attachment-chip inline-flex max-w-64 select-none items-center gap-1 whitespace-nowrap rounded-[var(--ui-radius-sm)] ui-bg-surface-100 px-[0.35em] align-bottom';

export abstract class EditorChip {
    public static isChip(n: Node): n is HTMLElement {
        const classList = n.nodeType === 1 ? (n as HTMLElement).classList : undefined;
        return (
            !!classList &&
            (classList.contains('mention-chip') ||
                classList.contains('attachment-chip') ||
                classList.contains('issue-link-chip'))
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

    public static buildIssueLinkChip(ref: IssueLinkRef, title?: string): HTMLElement {
        const s = document.createElement('span');
        s.className = 'issue-link-chip';
        s.contentEditable = 'false';
        s.dataset['token'] = IssueLinkConverter.toLinkMarkdown(ref);
        s.textContent = `#${ref.idIssuePublic}`;
        if (title) {
            s.title = title;
        }
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

    public static placeCaretBeside(root: HTMLElement, chip: HTMLElement, clientX: number): void {
        const box = chip.getBoundingClientRect();
        const range = root.ownerDocument.createRange();
        if (clientX < box.left + box.width / 2) {
            range.setStartBefore(chip);
        } else {
            range.setStartAfter(chip);
        }
        range.collapse(true);
        root.focus();
        const sel = root.ownerDocument.getSelection();
        sel?.removeAllRanges();
        sel?.addRange(range);
    }

    public static deleteChip(root: HTMLElement, isBackward: boolean): boolean {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount || !sel.isCollapsed) {
            return false;
        }
        const { startContainer: container, startOffset: offset } = sel.getRangeAt(0);
        if (!root.contains(container)) {
            return false;
        }
        const isText = container.nodeType === Node.TEXT_NODE;
        const chip = isBackward
            ? isText
                ? offset === 0
                    ? container.previousSibling
                    : null
                : (container.childNodes[offset - 1] ?? null)
            : isText
              ? offset === (container as Text).length
                  ? container.nextSibling
                  : null
              : (container.childNodes[offset] ?? null);
        if (!chip || !EditorChip.isChip(chip)) {
            return false;
        }
        const range = root.ownerDocument.createRange();
        range.setStartBefore(chip);
        range.collapse(true);
        chip.remove();
        sel.removeAllRanges();
        sel.addRange(range);
        return true;
    }

    // Chrome deletes the chip instead of a line break next to it, so that line break is removed by hand.
    public static deleteBesideChip(root: HTMLElement, isBackward: boolean): boolean {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount || !sel.isCollapsed) {
            return false;
        }
        const { startContainer: container, startOffset: offset } = sel.getRangeAt(0);
        const isText = container.nodeType === Node.TEXT_NODE;
        const before = isText
            ? offset > 0
                ? container
                : container.previousSibling
            : (container.childNodes[offset - 1] ?? null);
        const after = isText
            ? offset < (container as Text).length
                ? container
                : container.nextSibling
            : (container.childNodes[offset] ?? null);
        const chip = isBackward ? after : before;
        const target = isBackward ? before : after;
        if (!chip || !target || chip === target || !EditorChip.isChip(chip)) {
            return false;
        }
        if (target.nodeType === Node.TEXT_NODE) {
            const text = target as Text;
            const index = isBackward ? text.length - 1 : 0;
            if (text.data[index] !== '\n') {
                return false;
            }
            text.deleteData(index, 1);
            if (!text.length) {
                text.remove();
            }
        } else if (target instanceof HTMLBRElement) {
            target.remove();
        } else {
            return false;
        }
        const range = root.ownerDocument.createRange();
        if (isBackward) {
            range.setStartBefore(chip);
        } else {
            range.setStartAfter(chip);
        }
        range.collapse(true);
        sel.removeAllRanges();
        sel.addRange(range);
        return true;
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

    public static insertIssueLinkChipAtCaret(
        root: HTMLElement,
        ref: IssueLinkRef,
        title: string
    ): void {
        const sel = root.ownerDocument.getSelection();
        if (!sel?.rangeCount) return;
        EditorChip.insertNodeAtRange(
            root,
            sel,
            sel.getRangeAt(0),
            EditorChip.buildIssueLinkChip(ref, title)
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
