import { describe, expect, it } from 'vitest';
import { UiColor } from './ui-color';

describe('UiColor.normalizeHex', () => {
    it('passes through a canonical #rrggbb value', () => {
        expect(UiColor.normalizeHex('#6b7280')).toBe('#6b7280');
    });

    it('adds a missing leading #', () => {
        expect(UiColor.normalizeHex('6b7280')).toBe('#6b7280');
    });

    it('expands #rgb shorthand to #rrggbb', () => {
        expect(UiColor.normalizeHex('#abc')).toBe('#aabbcc');
    });

    it('expands rgb shorthand without #', () => {
        expect(UiColor.normalizeHex('fff')).toBe('#ffffff');
    });

    it('lowercases and trims', () => {
        expect(UiColor.normalizeHex('  #E57373  ')).toBe('#e57373');
    });

    it('falls back for named colours', () => {
        expect(UiColor.normalizeHex('red')).toBe('#000000');
    });

    it('falls back for rgb()/hsl() strings', () => {
        expect(UiColor.normalizeHex('rgb(255,0,0)')).toBe('#000000');
    });

    it('falls back for malformed hex (wrong length)', () => {
        expect(UiColor.normalizeHex('#12345')).toBe('#000000');
    });

    it('falls back for non-hex characters', () => {
        expect(UiColor.normalizeHex('#gggggg')).toBe('#000000');
    });

    it('uses the provided fallback', () => {
        expect(UiColor.normalizeHex(null, '#6b7280')).toBe('#6b7280');
        expect(UiColor.normalizeHex('nonsense', '#6b7280')).toBe('#6b7280');
    });
});
