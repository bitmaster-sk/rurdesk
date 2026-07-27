import { Color } from './color';

describe('Color.getContrastColor', () => {
    it('returns dark text on light backgrounds', () => {
        expect(Color.getContrastColor('#ffffff')).toBe('#1f2937');
        expect(Color.getContrastColor('#f0f0f0')).toBe('#1f2937');
    });

    it('returns light text on dark backgrounds', () => {
        expect(Color.getContrastColor('#000000')).toBe('#ffffff');
        expect(Color.getContrastColor('#1f2937')).toBe('#ffffff');
    });

    it('works with or without the leading #', () => {
        expect(Color.getContrastColor('ffffff')).toBe('#1f2937');
    });

    it('falls back to dark text for an invalid hex', () => {
        expect(Color.getContrastColor('not-a-color')).toBe('#1f2937');
    });
});

describe('Color.randomAvatarBg', () => {
    it('returns a canonical #rrggbb hex string', () => {
        for (let i = 0; i < 200; i++) {
            expect(Color.randomAvatarBg()).toMatch(/^#[0-9a-f]{6}$/);
        }
    });

    it('varies the hue across calls', () => {
        const colors = new Set(Array.from({ length: 30 }, () => Color.randomAvatarBg()));
        expect(colors.size).toBeGreaterThan(1);
    });
});
