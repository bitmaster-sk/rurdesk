import { lexer, type Token, type Tokens } from 'marked';
import { MessageSegmentParser } from './message-segment.parser';

/** A span of a text segment's source: offsets into the content, lines into the whole body. */
export interface MarkdownSpan {
    startOffset: number;
    endOffset: number;
    lineStart: number;
    lineEnd: number;
}

/** A top-level Markdown block. A top-level list also carries its items. */
export interface MarkdownBlock extends MarkdownSpan {
    /** False for blank lines and link definitions, which render nothing. */
    hasDom: boolean;
    items: MarkdownSpan[];
    /** Raw source of a link reference definition, empty otherwise. */
    definition: string;
}

const NO_DOM_TYPES = new Set(['space', 'def']);

export abstract class MarkdownBlockMap {
    /** Maps top-level Markdown blocks and list items to 1-based source lines starting at lineStart. */
    public static map(content: string, lineStart: number): MarkdownBlock[] {
        const body = content.replace(/\r\n?/g, '\n');
        const blocks: MarkdownBlock[] = [];
        let offset = 0;
        for (const token of lexer(body)) {
            const span = MarkdownBlockMap.span(body, lineStart, offset, token.raw);
            blocks.push({
                ...span,
                hasDom: !NO_DOM_TYPES.has(token.type),
                items: MarkdownBlockMap.listItems(body, lineStart, offset, token),
                definition: token.type === 'def' ? token.raw : ''
            });
            offset = span.endOffset;
        }
        return blocks;
    }

    private static listItems(
        body: string,
        lineStart: number,
        listOffset: number,
        token: Token
    ): MarkdownSpan[] {
        if (token.type !== 'list') {
            return [];
        }
        const items: MarkdownSpan[] = [];
        let offset = listOffset;
        for (const item of (token as Tokens.List).items) {
            const span = MarkdownBlockMap.span(body, lineStart, offset, item.raw);
            items.push(span);
            offset = span.endOffset;
        }
        return items;
    }

    /**
     * A block ends on the line of its last character that is not a trailing newline. A run of
     * blank lines starts after the newline that ends the previous block's line.
     */
    private static span(
        body: string,
        lineStart: number,
        offset: number,
        raw: string
    ): MarkdownSpan {
        const endOffset = offset + raw.length;
        const contentLength = raw.replace(/\n+$/, '').length;
        const toBodyLine = (at: number): number =>
            lineStart + MessageSegmentParser.lineAt(body, at) - 1;
        if (contentLength === 0) {
            const lineEnd = toBodyLine(endOffset - 1);
            return {
                startOffset: offset,
                endOffset,
                lineStart: Math.min(toBodyLine(offset + 1), lineEnd),
                lineEnd
            };
        }
        return {
            startOffset: offset,
            endOffset,
            lineStart: toBodyLine(offset),
            lineEnd: toBodyLine(offset + contentLength - 1)
        };
    }
}
