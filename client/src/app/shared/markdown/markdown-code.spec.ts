import { MarkdownCode } from './markdown-code';

describe('MarkdownCode', () => {
    it('returns only the text outside inline code and fenced blocks', () => {
        expect(MarkdownCode.outsideCode('a `x` b ```\ny\n``` c')).toEqual(['a ', ' b ', ' c']);
    });

    it('replaces text outside code and keeps code untouched', () => {
        expect(MarkdownCode.replaceOutsideCode('a `a` a', part => part.toUpperCase())).toBe(
            'A `a` A'
        );
    });
});
