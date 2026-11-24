import { ANCHOR_LINE_START, ANCHOR_SKIP_SELECTOR, AnchorDom } from './anchor-dom';

/** Upper bound the API accepts for a quote, in characters. */
const MAX_QUOTE_LENGTH = 2000;

/** Selected text of a rendered message, mapped back to its source lines. */
export interface AnchorSelection {
    lineStart: number;
    lineEnd: number;
    quote: string;
    /** Viewport rectangle of the selection, to place the reply button. */
    rect: DOMRect;
}

export abstract class AnchorSelectionReader {
    /**
     * Reads the selection inside host and maps it to the first and last touched block lines.
     * Returns null when empty, outside host, or touching a blocked or excluded element.
     */
    public static read(
        selection: Selection | null,
        host: HTMLElement,
        excludedSelector: string
    ): AnchorSelection | null {
        if (!selection || selection.isCollapsed || selection.rangeCount !== 1) {
            return null;
        }
        const range = selection.getRangeAt(0);
        if (!host.contains(range.commonAncestorContainer)) {
            return null;
        }
        const touches = (selector: string): boolean =>
            Array.from(host.querySelectorAll(selector)).some(element =>
                range.intersectsNode(element)
            ) ||
            AnchorSelectionReader.elementOf(range.commonAncestorContainer)?.closest(selector) !=
                null;
        if (touches('[data-anchor-blocked]') || touches(excludedSelector)) {
            return null;
        }

        const startBlock = AnchorSelectionReader.blockOf(range.startContainer, host);
        const endBlock = AnchorSelectionReader.blockOf(range.endContainer, host);
        if (!startBlock || !endBlock) {
            return null;
        }
        const start = AnchorDom.linesOf(startBlock);
        const end = AnchorDom.linesOf(endBlock);

        const fragment = range.cloneContents();
        fragment.querySelectorAll(ANCHOR_SKIP_SELECTOR).forEach(node => node.remove());
        const quote = AnchorDom.normalizeQuote(
            Array.from(AnchorDom.normalizeQuote(fragment.textContent ?? ''))
                .slice(0, MAX_QUOTE_LENGTH)
                .join('')
        );
        if (!quote) {
            return null;
        }
        return {
            lineStart: Math.min(start.lineStart, end.lineStart),
            lineEnd: Math.max(start.lineEnd, end.lineEnd),
            quote,
            rect: range.getBoundingClientRect()
        };
    }

    private static elementOf(node: Node): Element | null {
        return node instanceof Element ? node : node.parentElement;
    }

    private static blockOf(node: Node, host: HTMLElement): Element | null {
        const block =
            AnchorSelectionReader.elementOf(node)?.closest(`[${ANCHOR_LINE_START}]`) ?? null;
        return block && host.contains(block) ? block : null;
    }
}
