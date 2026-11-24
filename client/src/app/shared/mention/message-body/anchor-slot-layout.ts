import { ANCHOR_LINE_START, ANCHOR_SKIP_SELECTOR, ANCHOR_THREAD } from './anchor-dom';

const GAP_CLASS = 'anchor-gap';
/** Set on each thread, and on a draft, in the slot. */
const SLOT_THREAD = 'data-thread';
export const ARROW_LEFT_VAR = '--anchor-arrow-left';
/** Distance from the arrow's left edge to its tip; keeps the arrow off the rounded corners too. */
const ARROW_TIP = 14;

/**
 * Inserts a block gap after the visual line where the slot's last mark ends and overlays the slot
 * over it. Points each thread's arrow at the end of its highlighted text.
 */
export abstract class AnchorSlotLayout {
    public static apply(host: HTMLElement, slot: HTMLElement): void {
        AnchorSlotLayout.reset(host, slot);
        const threads = Array.from(slot.querySelectorAll<HTMLElement>(`[${SLOT_THREAD}]`));
        if (threads.length === 0) {
            return;
        }
        const marks = AnchorSlotLayout.marksOf(
            host,
            threads.map(thread => thread.getAttribute(SLOT_THREAD))
        );
        const lastMark = marks.at(-1);
        const split = lastMark ? AnchorSlotLayout.nextLineStart(lastMark) : null;
        if (split?.parentNode) {
            const gap = document.createElement('span');
            gap.className = GAP_CLASS;
            gap.style.display = 'block';
            gap.style.height = `${slot.offsetHeight}px`;
            split.parentNode.insertBefore(gap, split);
            AnchorSlotLayout.overlay(slot, gap);
        }
        for (const thread of threads) {
            AnchorSlotLayout.pointArrow(host, thread);
        }
    }

    public static reset(host: HTMLElement, slot: HTMLElement | null): void {
        for (const gap of Array.from(host.querySelectorAll(`.${GAP_CLASS}`))) {
            const parent = gap.parentNode;
            gap.remove();
            parent?.normalize();
        }
        if (slot) {
            slot.style.position = '';
            slot.style.top = '';
            slot.style.left = '';
            slot.style.right = '';
        }
    }

    private static overlay(slot: HTMLElement, gap: HTMLElement): void {
        const parent = slot.offsetParent;
        if (!(parent instanceof HTMLElement)) {
            return;
        }
        const parentRect = parent.getBoundingClientRect();
        slot.style.position = 'absolute';
        slot.style.left = '0';
        slot.style.right = '0';
        slot.style.top = `${gap.getBoundingClientRect().top - parentRect.top - parent.clientTop}px`;
    }

    private static pointArrow(host: HTMLElement, thread: HTMLElement): void {
        const rects = AnchorSlotLayout.marksOf(host, [thread.getAttribute(SLOT_THREAD)])
            .at(-1)
            ?.getClientRects();
        const end = rects?.[rects.length - 1];
        if (!end) {
            thread.style.removeProperty(ARROW_LEFT_VAR);
            return;
        }
        const box = thread.getBoundingClientRect();
        const left = Math.min(Math.max(end.right - box.left - ARROW_TIP, 8), box.width - 28);
        thread.style.setProperty(ARROW_LEFT_VAR, `${Math.round(left)}px`);
    }

    private static marksOf(host: HTMLElement, idsThread: (string | null)[]): HTMLElement[] {
        return Array.from(host.querySelectorAll<HTMLElement>(`mark[${ANCHOR_THREAD}]`)).filter(
            mark => idsThread.includes(mark.getAttribute(ANCHOR_THREAD))
        );
    }

    /**
     * The node the next visual line of the mark's block starts with, split out of its text node
     * when the line starts inside one; null when the mark ends on the block's last line.
     */
    private static nextLineStart(mark: HTMLElement): Node | null {
        const block = mark.parentElement?.closest<HTMLElement>(`[${ANCHOR_LINE_START}]`);
        const rects = mark.getClientRects();
        const last = rects[rects.length - 1];
        if (!block || !last) {
            return null;
        }
        const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT, {
            acceptNode: node =>
                node.parentElement?.closest(ANCHOR_SKIP_SELECTOR) ||
                mark.contains(node) ||
                !(mark.compareDocumentPosition(node) & Node.DOCUMENT_POSITION_FOLLOWING)
                    ? NodeFilter.FILTER_REJECT
                    : NodeFilter.FILTER_ACCEPT
        });
        const range = document.createRange();
        for (
            let node = walker.nextNode() as Text | null;
            node;
            node = walker.nextNode() as Text | null
        ) {
            for (let offset = 0; offset < node.data.length; offset++) {
                if (/\s/.test(node.data[offset])) {
                    continue;
                }
                range.setStart(node, offset);
                range.setEnd(node, offset + 1);
                if (range.getBoundingClientRect().top > last.top + last.height / 2) {
                    return AnchorSlotLayout.splitAt(node, offset, block);
                }
            }
        }
        return null;
    }

    private static splitAt(node: Text, offset: number, block: HTMLElement): Node {
        if (offset > 0) {
            return node.splitText(offset);
        }
        // A line that starts with an inline element, such as a chip, gets the gap before it.
        let start: Node = node;
        while (
            start.parentNode &&
            start.parentNode !== block &&
            start.parentNode.firstChild === start
        ) {
            start = start.parentNode;
        }
        return start;
    }
}
