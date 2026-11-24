import { MarkdownBlockMap, MarkdownSpan } from './markdown-block-map';
import { MessageSegmentParser } from './message-segment.parser';

/** A piece of a text segment rendered on its own, followed by the threads that end inside it. */
export interface AnchorChunk {
    content: string;
    lineStart: number;
    lineEnd: number;
    /** Body lines of the thread ranges that end in this chunk; their threads render after it. */
    breaks: number[];
}

export abstract class AnchorChunker {
    /**
     * Splits a text segment after the block holding each break line (and between top-level list
     * items). Copies link definitions into each chunk and drops a trailing blank chunk.
     */
    public static split(content: string, lineStart: number, breaks: number[]): AnchorChunk[] {
        const body = content.replace(/\r\n?/g, '\n');
        const blocks = MarkdownBlockMap.map(body, lineStart);
        const units: MarkdownSpan[] = blocks.flatMap(block =>
            block.items.length > 0 ? block.items : [block]
        );
        const definitions = blocks
            .filter(block => block.definition)
            .map(block => block.definition.trimEnd())
            .join('\n');

        let pending = [...breaks].sort((a, b) => a - b);
        const chunks: AnchorChunk[] = [];
        let chunkStart = 0;
        let chunkLineStart = lineStart;
        units.forEach((unit, index) => {
            const isLast = index === units.length - 1;
            const owned = pending.filter(line => isLast || line <= unit.lineEnd);
            if (owned.length === 0) {
                return;
            }
            pending = pending.filter(line => !owned.includes(line));
            chunks.push({
                content: body.slice(chunkStart, unit.endOffset),
                lineStart: chunkLineStart,
                lineEnd: unit.lineEnd,
                breaks: owned
            });
            chunkStart = unit.endOffset;
            // The next chunk starts with the newline that ends this unit's last line.
            chunkLineStart = lineStart + MessageSegmentParser.lineAt(body, chunkStart) - 1;
        });

        const rest = body.slice(chunkStart);
        if (chunks.length === 0 || rest.trim().length > 0) {
            chunks.push({
                content: rest,
                lineStart: chunkLineStart,
                lineEnd: units.at(-1)?.lineEnd ?? lineStart,
                breaks: pending
            });
        }
        if (chunks.length > 1 && definitions) {
            for (const chunk of chunks) {
                chunk.content = `${chunk.content.replace(/\n*$/, '')}\n\n${definitions}\n`;
            }
        }
        return chunks;
    }
}
