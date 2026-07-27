import { extractMessageSegments } from './extract-message-segments';

describe('extractMessageSegments', () => {
    it('returns a single text segment when body has no fence', () => {
        const out = extractMessageSegments('plain markdown content');
        expect(out).toEqual([{ type: 'text', content: 'plain markdown content' }]);
    });

    it('returns one empty text segment for empty input', () => {
        expect(extractMessageSegments('')).toEqual([{ type: 'text', content: '' }]);
    });

    it('splits a single diff fence between text on both sides', () => {
        const body =
            'Before text\n' +
            '```diff\n' +
            '--- a/x\n+++ b/x\n@@ @@\n+added\n' +
            '```\n' +
            'After text';
        const out = extractMessageSegments(body);
        expect(out.length).toBe(3);
        expect(out[0]).toEqual({ type: 'text', content: 'Before text\n' });
        expect(out[1].type).toBe('diff');
        expect(out[1].content).toContain('--- a/x');
        expect(out[1].content).toContain('+added');
        expect(out[2]).toEqual({ type: 'text', content: '\nAfter text' });
    });

    it('handles two consecutive diff fences without dropping either', () => {
        const body =
            '```diff\n' +
            '--- a/one\n+++ b/one\n@@ @@\n+a\n' +
            '```\n' +
            '\n' +
            '```diff\n' +
            '--- a/two\n+++ b/two\n@@ @@\n+b\n' +
            '```';
        const out = extractMessageSegments(body);
        const diffs = out.filter(s => s.type === 'diff');
        expect(diffs.length).toBe(2);
        expect(diffs[0].content).toContain('+a');
        expect(diffs[1].content).toContain('+b');
    });

    it('tolerates extra info on the opening fence (e.g. ```diff go)', () => {
        const body = '```diff go\n--- a/f\n+++ b/f\n@@ @@\n+x\n```';
        const out = extractMessageSegments(body);
        expect(out.length).toBe(1);
        expect(out[0].type).toBe('diff');
        expect(out[0].content).toContain('+x');
    });

    it('keeps non-recognized fences as text content', () => {
        const body =
            'Look:\n' +
            '```go\nfunc x() {}\n```\n' +
            'and a diff:\n' +
            '```diff\n--- a/f\n+++ b/f\n@@ @@\n+y\n```';
        const out = extractMessageSegments(body);
        // 2 segments: text (containing the go fence intact) + diff
        expect(out.length).toBe(2);
        expect(out[0].type).toBe('text');
        expect(out[0].content).toContain('```go');
        expect(out[0].content).toContain('func x()');
        expect(out[1].type).toBe('diff');
    });

    it('extracts a mockup fence as a mockup segment carrying the raw html', () => {
        const body = 'Here is a mockup:\n```mockup\n<h1>Hi</h1>\n```';
        const out = extractMessageSegments(body);
        expect(out.length).toBe(2);
        expect(out[0].type).toBe('text');
        expect(out[1]).toEqual({
            type: 'mockup',
            content: '<h1>Hi</h1>',
            title: undefined,
            ref: '#1'
        });
        // (no title → ref is the bare index)
    });

    it('parses the title="…" attribute on a mockup fence', () => {
        const body = '```mockup title="Login screen"\n<button>Go</button>\n```';
        const out = extractMessageSegments(body);
        expect(out.length).toBe(1);
        expect(out[0].type).toBe('mockup');
        expect(out[0].title).toBe('Login screen');
        expect(out[0].content).toBe('<button>Go</button>');
    });

    it('splits a body mixing a diff and a mockup in order', () => {
        const body =
            '```diff\n--- a/f\n+++ b/f\n@@ @@\n+x\n```\n' + 'then\n' + '```mockup\n<p>m</p>\n```';
        const out = extractMessageSegments(body);
        expect(out.map(s => s.type)).toEqual(['diff', 'text', 'mockup']);
        expect(out[2].content).toBe('<p>m</p>');
    });

    describe('mockup ref', () => {
        it('builds ref from the title plus 1-based order when present', () => {
            const body = 'A\n```mockup title="Login"\n<p>x</p>\n```\n';
            const seg = extractMessageSegments(body).find(s => s.type === 'mockup')!;
            expect(seg.ref).toBe('Login #1');
            expect(seg.title).toBe('Login'); // title stays clean for display
        });

        it('falls back to #N in mockup order when no title', () => {
            const body = '```mockup\n<p>a</p>\n```\ntext\n```mockup\n<p>b</p>\n```\n';
            const refs = extractMessageSegments(body)
                .filter(s => s.type === 'mockup')
                .map(s => s.ref);
            expect(refs).toEqual(['#1', '#2']);
        });

        it('keeps refs unique when two mockups share a title', () => {
            const body =
                '```mockup title="Login"\n<p>a</p>\n```\n```mockup title="Login"\n<p>b</p>\n```\n';
            const refs = extractMessageSegments(body)
                .filter(s => s.type === 'mockup')
                .map(s => s.ref);
            expect(refs).toEqual(['Login #1', 'Login #2']);
            expect(new Set(refs).size).toBe(2);
        });
    });
});
