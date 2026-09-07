import { AsciiEmoji } from './ascii-emoji';

describe('AsciiEmoji.matchBeforeCaret', () => {
    it('matches a shortcut the user just terminated with a space', () => {
        expect(AsciiEmoji.matchBeforeCaret(':) ')).toEqual({ shortcut: ':)', emoji: '😄' });
        expect(AsciiEmoji.matchBeforeCaret('hi <3 ')).toEqual({ shortcut: '<3', emoji: '💗' });
    });

    it('does not match until the terminating space is typed', () => {
        expect(AsciiEmoji.matchBeforeCaret(':)')).toBeNull();
        expect(AsciiEmoji.matchBeforeCaret('hi :-D')).toBeNull();
    });

    it('requires whitespace or start of text before the shortcut', () => {
        expect(AsciiEmoji.matchBeforeCaret('const x = {a:P} ')).toBeNull();
        expect(AsciiEmoji.matchBeforeCaret('let p:Promise ')).toBeNull();
        expect(AsciiEmoji.matchBeforeCaret('https://x.com/a:O ')).toBeNull();
        expect(AsciiEmoji.matchBeforeCaret('{name:props.name} ')).toBeNull();
    });

    it('matches after a newline as well as after a space', () => {
        expect(AsciiEmoji.matchBeforeCaret('first line\n;) ')).toEqual({
            shortcut: ';)',
            emoji: '😉'
        });
    });

    it('prefers the longest shortcut when one is a suffix of another', () => {
        expect(AsciiEmoji.matchBeforeCaret('>:) ')).toEqual({ shortcut: '>:)', emoji: '😈' });
        expect(AsciiEmoji.matchBeforeCaret('>:-) ')).toEqual({ shortcut: '>:-)', emoji: '😈' });
    });

    it('does not match a shortcut glued to the preceding word', () => {
        expect(AsciiEmoji.matchBeforeCaret('a>:) ')).toBeNull();
    });

    it('only looks at the character run ending at the caret', () => {
        expect(AsciiEmoji.matchBeforeCaret(':) already converted, now typing ')).toBeNull();
    });

    it('carries no single-letter shortcuts, which collided with code', () => {
        for (const glued of [':b ', ':p ', ':o ', ':-b ', ':-p ', ':-o ']) {
            expect(AsciiEmoji.matchBeforeCaret(glued)).toBeNull();
        }
    });

    it('returns null for text with no shortcut at all', () => {
        expect(AsciiEmoji.matchBeforeCaret('')).toBeNull();
        expect(AsciiEmoji.matchBeforeCaret('no shortcut here ')).toBeNull();
    });
});
