import { describe, it, expect } from 'vitest';
import { parseQuery } from './parse-query.util';

describe('parseQuery', () => {
    it('all mode with trimmed query when no prefix', () => {
        expect(parseQuery('  login bug ')).toEqual({ mode: 'all', query: 'login bug' });
    });
    it('maps > @ # / to their modes', () => {
        expect(parseQuery('> set')).toEqual({ mode: 'commands', query: 'set' });
        expect(parseQuery('@petra')).toEqual({ mode: 'people', query: 'petra' });
        expect(parseQuery('#428')).toEqual({ mode: 'issues', query: '428' });
        expect(parseQuery('/board')).toEqual({ mode: 'navigation', query: 'board' });
    });
    it('lone prefix → that mode, empty query', () => {
        expect(parseQuery('/')).toEqual({ mode: 'navigation', query: '' });
    });
});
