/**
 * A segment of a markdown message body after splitting out fenced blocks that
 * get their own renderer. Text segments retain their markdown content and are
 * rendered via the markdown directive; `diff` segments carry the raw unified-
 * diff string (without fences) rendered via `<app-diff-viewer [rawPatch]>`;
 * `mockup` segments carry raw HTML rendered via `<app-mockup-card [html]>`;
 * `mermaid` segments carry the diagram source (without fences) rendered via
 * `<app-mermaid-diagram [source]>`.
 */
export interface MessageSegment {
    type: 'text' | 'diff' | 'mockup' | 'mermaid';
    content: string;
    /** Optional `title="…"` from a ```mockup fence info string. */
    title?: string;
    /** Stable reference for a mockup segment: its title, else `#N` (1-based
     * order among mockups in the message). Used to approve a specific mockup. */
    ref?: string;
}

/** Which fence kinds a parse call recognizes; see `MessageSegmentParser.parse`. */
export type FenceFilter = 'all' | 'mermaid';

// Matches a fenced code block tagged `diff`, `mockup` or `mermaid`. The opening
// fence may carry extra info (e.g. `diff lang=go`, `mockup title="Login"`); we
// only require the tag to start the info string. Newlines around the body are
// captured non-greedily so adjacent segments don't merge.
const FENCE_PATTERN = /```(diff|mockup|mermaid)([^\n]*)\n([\s\S]*?)\n```/;

// Matches only mermaid fences — user comments split on diagrams alone, so
// ```diff and ```mockup fences keep rendering as plain code blocks there
// (the diff viewer and mockup card stay agent-message features).
const MERMAID_FENCE_PATTERN = /```(mermaid)([^\n]*)\n([\s\S]*?)\n```/;

export abstract class MessageSegmentParser {
    /**
     * Split a markdown body into alternating text / diff / mockup / mermaid
     * segments. Agent messages are markdown that may contain one or more
     * ```diff, ```mockup or ```mermaid fenced blocks; the activity feed renders
     * each segment with the appropriate component (markdown for text, diff
     * viewer for diff, mockup card for mockup, mermaid diagram for mermaid).
     *
     * `fences: 'mermaid'` recognizes only mermaid fences — used for user
     * comments and chat messages, where diagrams are a first-class feature but
     * diff/mockup fences must stay plain code blocks.
     *
     * Empty text segments are dropped so the rendered output doesn't carry stray
     * gaps between consecutive fenced blocks. The method never returns fewer than
     * one segment: a body with no recognized fence becomes a single `text` segment
     * carrying the original string.
     */
    public static parse(body: string, fences: FenceFilter = 'all'): MessageSegment[] {
        if (!body) {
            return [{ type: 'text', content: '' }];
        }

        const segments: MessageSegment[] = [];
        let lastIndex = 0;
        let mockupNo = 0;
        // Fresh RegExp per call — the `g` flag mutates lastIndex on a shared
        // instance, which would corrupt concurrent calls (e.g. ChangeDetection
        // re-evaluating the computed for multiple messages in a single tick).
        const source = fences === 'mermaid' ? MERMAID_FENCE_PATTERN.source : FENCE_PATTERN.source;
        const pattern = new RegExp(source, 'g');
        let match: RegExpExecArray | null;

        while ((match = pattern.exec(body)) !== null) {
            const textBefore = body.slice(lastIndex, match.index);
            if (textBefore.length > 0) {
                segments.push({ type: 'text', content: textBefore });
            }

            const [, lang, info, content] = match;
            if (lang === 'mockup') {
                mockupNo++;
                const title = MessageSegmentParser.parseTitle(info);
                // ref must be unique within the message (two mockups can share a
                // title) — fold in the 1-based order. title stays untouched for
                // display; ref is only the selection key sent on approve.
                segments.push({
                    type: 'mockup',
                    content,
                    title,
                    ref: title ? `${title} #${mockupNo}` : `#${mockupNo}`
                });
            } else if (lang === 'mermaid') {
                segments.push({ type: 'mermaid', content });
            } else {
                segments.push({ type: 'diff', content });
            }

            lastIndex = match.index + match[0].length;
        }

        const tail = body.slice(lastIndex);
        if (tail.length > 0) {
            segments.push({ type: 'text', content: tail });
        }

        return segments;
    }

    /** Pull `title="…"` (or `title='…'`) out of a fence info string. */
    private static parseTitle(info: string): string | undefined {
        const match = /title\s*=\s*("([^"]*)"|'([^']*)')/.exec(info);
        return match ? (match[2] ?? match[3]) : undefined;
    }
}
