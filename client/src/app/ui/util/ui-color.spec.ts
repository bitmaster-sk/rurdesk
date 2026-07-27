import { describe, expect, it } from 'vitest';
import { normalizeHexColor } from './ui-color';

describe('normalizeHexColor', () => {
    it('passes through a canonical #rrggbb value', () => {
        expect(normalizeHexColor('#6b7280')).toBe('#6b7280');
    });

    it('adds a missing leading #', () => {
        expect(normalizeHexColor('6b7280')).toBe('#6b7280');
    });

    it('expands #rgb shorthand to #rrggbb', () => {
        expect(normalizeHexColor('#abc')).toBe('#aabbcc');
    });

    it('expands rgb shorthand without #', () => {
        expect(normalizeHexColor('fff')).toBe('#ffffff');
    });

    it('lowercases and trims', () => {
        expect(normalizeHexColor('  #E57373  ')).toBe('#e57373');
    });

    it('falls back for named colours', () => {
        expect(normalizeHexColor('red')).toBe('#000000');
    });

    it('falls back for rgb()/hsl() strings', () => {
        expect(normalizeHexColor('rgb(255,0,0)')).toBe('#000000');
    });

    it('falls back for malformed hex (wrong length)', () => {
        expect(normalizeHexColor('#12345')).toBe('#000000');
    });

    it('falls back for non-hex characters', () => {
        expect(normalizeHexColor('#gggggg')).toBe('#000000');
    });

    it('uses the provided fallback', () => {
        expect(normalizeHexColor(null, '#6b7280')).toBe('#6b7280');
        expect(normalizeHexColor('nonsense', '#6b7280')).toBe('#6b7280');
    });
});
