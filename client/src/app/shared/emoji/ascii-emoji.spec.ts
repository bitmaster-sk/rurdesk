import { fromAsciiToEmoji } from './ascii-emoji';

describe('fromAsciiToEmoji', () => {
    it('replaces known ascii smileys with emoji', () => {
        expect(fromAsciiToEmoji(':)')).toBe('😄');
        expect(fromAsciiToEmoji('<3')).toBe('💗');
    });

    it('replaces occurrences within surrounding text', () => {
        expect(fromAsciiToEmoji('hi <3 you ;)')).toBe('hi 💗 you 😉');
    });

    it('leaves text without ascii smileys unchanged', () => {
        expect(fromAsciiToEmoji('no emoji here')).toBe('no emoji here');
    });
});
