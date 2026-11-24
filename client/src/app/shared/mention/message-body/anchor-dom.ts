/** A thread's anchored text to highlight in a rendered message. */
export interface AnchorHighlight {
    idThread: number;
    lineStart: number;
    lineEnd: number;
    quote?: string;
}

export const ANCHOR_LINE_START = 'data-line-start';
export const ANCHOR_LINE_END = 'data-line-end';
export const ANCHOR_THREAD = 'data-anchor-thread';
/** Nodes that are UI chrome, not message text: skipped when reading or matching a quote. */
export const ANCHOR_SKIP_SELECTOR = '[data-markdown-code-copy], button';

interface TextPosition {
    node: Text;
    offset: number;
}

export abstract class AnchorDom {
    public static setLines(element: Element, lineStart: number, lineEnd: number): void {
        element.setAttribute(ANCHOR_LINE_START, String(lineStart));
        element.setAttribute(ANCHOR_LINE_END, String(lineEnd));
    }

    public static linesOf(element: Element): { lineStart: number; lineEnd: number } {
        return {
            lineStart: Number(element.getAttribute(ANCHOR_LINE_START)),
            lineEnd: Number(element.getAttribute(ANCHOR_LINE_END))
        };
    }

    public static normalizeQuote(text: string): string {
        return text.replace(/\s+/g, ' ').trim();
    }

    /** Wraps the first whitespace-insensitive match of quote in `<mark>`, one per text node. */
    public static wrapQuote(blocks: Element[], quote: string, idThread: number): boolean {
        const needle = quote.replace(/\s+/g, '');
        if (!needle) {
            return false;
        }
        const positions: TextPosition[] = [];
        let haystack = '';
        for (const block of blocks) {
            for (const node of AnchorDom.textNodes(block)) {
                const text = node.data;
                for (let offset = 0; offset < text.length; offset++) {
                    if (!/\s/.test(text[offset])) {
                        haystack += text[offset];
                        positions.push({ node, offset });
                    }
                }
            }
        }
        const start = haystack.indexOf(needle);
        if (start < 0) {
            return false;
        }
        const pieces = new Map<Text, { from: number; to: number }>();
        for (let index = start; index < start + needle.length; index++) {
            const { node, offset } = positions[index];
            pieces.set(node, { from: pieces.get(node)?.from ?? offset, to: offset + 1 });
        }
        // Each piece lives in its own text node, so wrapping one never shifts another's offsets.
        for (const [node, { from, to }] of pieces) {
            const range = document.createRange();
            range.setStart(node, from);
            range.setEnd(node, to);
            const mark = document.createElement('mark');
            mark.className = 'anchor-mark';
            mark.setAttribute(ANCHOR_THREAD, String(idThread));
            range.surroundContents(mark);
        }
        return true;
    }

    private static textNodes(root: Element): Text[] {
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
            acceptNode: node =>
                node.parentElement?.closest(ANCHOR_SKIP_SELECTOR)
                    ? NodeFilter.FILTER_REJECT
                    : NodeFilter.FILTER_ACCEPT
        });
        const nodes: Text[] = [];
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            nodes.push(node as Text);
        }
        return nodes;
    }
}
