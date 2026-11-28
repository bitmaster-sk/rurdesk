import { describe, expect, it } from 'vitest';
import { Abbreviation } from './abbreviation';

describe('Abbreviation.of', () => {
    it('takes the first two letters of a one-word name', () => {
        expect(Abbreviation.of('rurdesk')).toBe('RU');
    });

    it('takes the initials of the first two words', () => {
        expect(Abbreviation.of('Agent VL')).toBe('AV');
        expect(Abbreviation.of('IPSOM & Infra')).toBe('II');
    });

    it('keeps accented letters', () => {
        expect(Abbreviation.of('časovač')).toBe('ČA');
    });

    it('falls back to a question mark for an empty name', () => {
        expect(Abbreviation.of('')).toBe('?');
        expect(Abbreviation.of('   ')).toBe('?');
    });
});
