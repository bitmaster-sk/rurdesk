import { lexer, type Token, type Tokens } from 'marked';

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
     * Fences are found with the marked lexer, so only top-level code blocks
     * count: a fence nested inside a list item or inside a longer fence stays
     * part of the surrounding text. Empty text segments are dropped so the
     * rendered output doesn't carry stray gaps between consecutive fenced
     * blocks. The method never returns fewer than one segment: a body with no
     * recognized fence becomes a single `text` segment carrying the original
     * string.
     */
    public static parse(body: string, fences: FenceFilter = 'all'): MessageSegment[] {
        if (!body) {
            return [{ type: 'text', content: '' }];
        }

        const segments: MessageSegment[] = [];
        let mockupNo = 0;
        // Token raws tile the source, so the running offset tracks each
        // token's start position in the body.
        let offset = 0;
        let textStart = 0;

        const pushText = (end: number): void => {
            const text = body.slice(textStart, end);
            if (text.length > 0) {
                segments.push({ type: 'text', content: text });
            }
        };

        for (const token of lexer(body)) {
            const fence = MessageSegmentParser.recognize(token, fences);
            const end = offset + token.raw.length;
            if (fence) {
                pushText(offset);
                if (fence.kind === 'mockup') {
                    mockupNo++;
                    const title = MessageSegmentParser.parseTitle(fence.info);
                    // ref must be unique within the message (two mockups can share a
                    // title) — fold in the 1-based order. title stays untouched for
                    // display; ref is only the selection key sent on approve.
                    segments.push({
                        type: 'mockup',
                        content: fence.content,
                        title,
                        ref: title ? `${title} #${mockupNo}` : `#${mockupNo}`
                    });
                } else {
                    segments.push({ type: fence.kind, content: fence.content });
                }
                // A recognized block ends at its closing fence: marked absorbs
                // the newline behind the fence into the token raw, but it
                // belongs to the text that follows — handing it back keeps
                // text boundaries identical to the source.
                textStart = end - (token.raw.endsWith('\n') ? 1 : 0);
            }
            offset = end;
        }

        pushText(body.length);
        return segments;
    }

    private static recognize(
        token: Token,
        fences: FenceFilter
    ): { kind: 'diff' | 'mockup' | 'mermaid'; info: string; content: string } | null {
        if (token.type !== 'code') {
            return null;
        }
        const code = token as Tokens.Code;
        const lang = code.lang ?? '';
        const kind = lang.split(/\s+/)[0] as 'diff' | 'mockup' | 'mermaid';
        const allowed =
            fences === 'mermaid'
                ? kind === 'mermaid'
                : kind === 'diff' || kind === 'mockup' || kind === 'mermaid';
        if (!allowed) {
            return null;
        }
        return { kind, info: lang.slice(kind.length), content: code.text };
    }

    /** Pull `title="…"` (or `title='…'`) out of a fence info string. */
    private static parseTitle(info: string): string | undefined {
        const match = /title\s*=\s*("([^"]*)"|'([^']*)')/.exec(info);
        return match ? (match[2] ?? match[3]) : undefined;
    }
}
