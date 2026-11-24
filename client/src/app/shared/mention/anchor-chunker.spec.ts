import { describe, it, expect } from 'vitest';
import { marked } from 'marked';
import { MarkdownBlockMap } from './markdown-block-map';
import { AnchorChunker } from './anchor-chunker';
import { MessageSegmentParser } from './message-segment.parser';

const linesOf = (content: string, lineStart = 1): [number, number][] =>
    MarkdownBlockMap.map(content, lineStart)
        .filter(block => block.hasDom)
        .map(block => [block.lineStart, block.lineEnd]);

describe('MarkdownBlockMap.map', () => {
    it('gives each kind of block its source lines', () => {
        const body = [
            '# Heading', // 1
            '', // 2
            'A paragraph', // 3
            'on two lines', // 4
            '', // 5
            '```js', // 6
            'code()', // 7
            '```', // 8
            '> quoted', // 9
            '> more', // 10
            '', // 11
            '<div>', // 12
            'html', // 13
            '</div>' // 14
        ].join('\n');
        expect(linesOf(body)).toEqual([
            [1, 1],
            [3, 4],
            [6, 8],
            [9, 10],
            [12, 14]
        ]);
    });

    it('maps the items of a top-level list', () => {
        const body = 'Intro\n\n1. one\n2. two\n   - nested\n3. three';
        const list = MarkdownBlockMap.map(body, 1).find(block => block.items.length > 0)!;
        expect(list.items.map(item => [item.lineStart, item.lineEnd])).toEqual([
            [3, 3],
            [4, 5],
            [6, 6]
        ]);
    });

    it('renders nothing for a link definition and keeps its source', () => {
        const blocks = MarkdownBlockMap.map('See [docs].\n\n[docs]: https://example.com', 1);
        const definition = blocks.find(block => block.definition)!;
        expect(definition.hasDom).toBe(false);
        expect(definition.definition).toContain('[docs]: https://example.com');
    });

    it('counts a CRLF body like an LF body', () => {
        expect(linesOf('# A\r\n\r\npara\r\nmore\r\n')).toEqual(linesOf('# A\n\npara\nmore\n'));
    });

    it('offsets lines by the segment start', () => {
        expect(linesOf('para', 7)).toEqual([[7, 7]]);
    });
});

describe('MessageSegmentParser.parseLines', () => {
    it('gives segment lines after a mermaid fence', () => {
        const body = 'Intro\n```mermaid\ngraph TD\nA-->B\n```\nAfter\nend';
        const segments = MessageSegmentParser.parseLines(body, 'diagrams');
        expect(segments.map(s => [s.type, s.lineStart, s.lineEnd])).toEqual([
            ['text', 1, 1],
            ['mermaid', 2, 5],
            ['text', 5, 7]
        ]);
        expect(linesOf(segments[2].content, segments[2].lineStart)).toEqual([[6, 7]]);
    });

    it('does not count a trailing newline as a line, like the server', () => {
        const segments = MessageSegmentParser.parseLines('one\ntwo\n');
        expect(segments.at(-1)!.lineEnd).toBe(2);
    });
});

describe('AnchorChunker.split', () => {
    const list = '1. one\n2. two\n3. three\n4. four\n5. five';

    it('returns the whole content when nothing breaks it', () => {
        expect(AnchorChunker.split('a\n\nb', 1, [])).toEqual([
            { content: 'a\n\nb', lineStart: 1, lineEnd: 3, breaks: [] }
        ]);
    });

    it('splits a paragraph run after the block holding the break', () => {
        const chunks = AnchorChunker.split('first\n\nsecond\nstill second\n\nthird', 1, [3]);
        expect(chunks.map(chunk => [chunk.lineStart, chunk.lineEnd, chunk.breaks])).toEqual([
            [1, 4, [3]],
            [4, 6, []]
        ]);
        expect(chunks[0].content).toContain('still second');
    });

    it('continues an ordered list with its source numbers after a split', () => {
        const chunks = AnchorChunker.split(list, 1, [3]);
        expect(chunks).toHaveLength(2);
        expect(chunks[1].content.startsWith('4.')).toBe(true);
        expect(marked.parse(chunks[1].content)).toContain('<ol start="4">');
    });

    it('splits a nested list only at a top-level item', () => {
        const body = '1. one\n   - a\n   - b\n2. two\n3. three';
        const chunks = AnchorChunker.split(body, 1, [2]);
        expect(chunks[0].content).toBe('1. one\n   - a\n   - b\n');
        expect(chunks[0].breaks).toEqual([2]);
        expect(chunks[1].content.startsWith('2. two')).toBe(true);
    });

    it('produces no empty chunk for a thread on the last line', () => {
        const chunks = AnchorChunker.split('a\n\nb\n', 1, [3]);
        expect(chunks).toHaveLength(1);
        expect(chunks[0].breaks).toEqual([3]);
    });

    it('puts a break past the last block after it', () => {
        expect(AnchorChunker.split('a', 1, [9]).map(chunk => chunk.breaks)).toEqual([[9]]);
    });

    it('collects breaks of the same block into one chunk', () => {
        const chunks = AnchorChunker.split('a\nb\n\nc', 1, [2, 1]);
        expect(chunks[0].breaks).toEqual([1, 2]);
    });

    it('copies link definitions into every chunk', () => {
        const chunks = AnchorChunker.split(
            'See [docs].\n\nMore.\n\n[docs]: https://example.com',
            1,
            [1]
        );
        expect(chunks).toHaveLength(2);
        expect(marked.parse(chunks[0].content)).toContain('href="https://example.com"');
    });
});
