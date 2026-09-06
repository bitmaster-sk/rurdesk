import { describe, it, expect } from 'vitest';
import { FuzzySearch } from './fuzzy-search';

describe('FuzzySearch.match', () => {
    it('matches empty query against anything with score 0', () => {
        expect(FuzzySearch.match('', 'anything')).toEqual({ matched: true, score: 0 });
    });
    it('matches scattered characters in order', () => {
        expect(FuzzySearch.match('lgn', 'Login bug').matched).toBe(true);
    });
    it('does not match when a character is missing', () => {
        expect(FuzzySearch.match('xyz', 'Login bug').matched).toBe(false);
    });
    it('is case-insensitive', () => {
        expect(FuzzySearch.match('LOGIN', 'login bug').matched).toBe(true);
    });
    it('scores a contiguous substring higher than a scattered match', () => {
        expect(FuzzySearch.match('log', 'log out').score).toBeGreaterThan(
            FuzzySearch.match('log', 'lots of goo').score
        );
    });
});

describe('FuzzySearch.highlight', () => {
    it('splits into hit and non-hit segments for matched chars', () => {
        expect(FuzzySearch.highlight('Login', 'lg')).toEqual([
            { text: 'L', hit: true },
            { text: 'o', hit: false },
            { text: 'g', hit: true },
            { text: 'in', hit: false }
        ]);
    });
    it('returns the whole text as one non-hit segment for empty query', () => {
        expect(FuzzySearch.highlight('Login', '')).toEqual([{ text: 'Login', hit: false }]);
    });
});
