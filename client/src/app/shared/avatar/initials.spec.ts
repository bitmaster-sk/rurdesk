import { Initials } from './initials';

describe('Initials.fromName', () => {
    it('returns the first letters of the first two words', () => {
        expect(Initials.fromName('Alice Smith')).toBe('AS');
    });

    it('returns the first two letters of a single-word name', () => {
        expect(Initials.fromName('Alice')).toBe('AL');
    });

    it('uses only the first two words of a three-word name', () => {
        expect(Initials.fromName('Alice Bob Charlie')).toBe('AB');
    });

    it('trims surrounding whitespace', () => {
        expect(Initials.fromName('  Alice Smith  ')).toBe('AS');
    });

    it('uppercases the result', () => {
        expect(Initials.fromName('alice smith')).toBe('AS');
    });

    it('returns an empty string for an empty name', () => {
        expect(Initials.fromName('')).toBe('');
    });

    it('returns an empty string for a whitespace-only name', () => {
        expect(Initials.fromName('   ')).toBe('');
    });
});
