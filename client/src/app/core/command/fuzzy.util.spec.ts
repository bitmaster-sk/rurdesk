import { describe, it, expect } from 'vitest';
import { fuzzyMatch, highlight } from './fuzzy.util';

describe('fuzzyMatch', () => {
    it('matches empty query against anything with score 0', () => {
        expect(fuzzyMatch('', 'anything')).toEqual({ matched: true, score: 0 });
    });
    it('matches scattered characters in order', () => {
        expect(fuzzyMatch('lgn', 'Login bug').matched).toBe(true);
    });
    it('does not match when a character is missing', () => {
        expect(fuzzyMatch('xyz', 'Login bug').matched).toBe(false);
    });
    it('is case-insensitive', () => {
        expect(fuzzyMatch('LOGIN', 'login bug').matched).toBe(true);
    });
    it('scores a contiguous substring higher than a scattered match', () => {
        expect(fuzzyMatch('log', 'log out').score).toBeGreaterThan(
            fuzzyMatch('log', 'lots of goo').score
        );
    });
});

describe('highlight', () => {
    it('splits into hit and non-hit segments for matched chars', () => {
        expect(highlight('Login', 'lg')).toEqual([
            { text: 'L', hit: true },
            { text: 'o', hit: false },
            { text: 'g', hit: true },
            { text: 'in', hit: false }
        ]);
    });
    it('returns the whole text as one non-hit segment for empty query', () => {
        expect(highlight('Login', '')).toEqual([{ text: 'Login', hit: false }]);
    });
});
