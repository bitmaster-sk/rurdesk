import { TokenFormatter } from './token.formatter';

describe('TokenFormatter.format', () => {
    it('shows small counts as they are', () => {
        expect(TokenFormatter.format(0)).toBe('0');
        expect(TokenFormatter.format(999)).toBe('999');
    });

    it('shows thousands with one decimal and k', () => {
        expect(TokenFormatter.format(1234)).toBe('1.2k');
        expect(TokenFormatter.format(412_534)).toBe('412.5k');
    });

    it('shows millions with one decimal and M', () => {
        expect(TokenFormatter.format(2_345_678)).toBe('2.3M');
    });
});
