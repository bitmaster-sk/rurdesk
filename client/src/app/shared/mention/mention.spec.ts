import { describe, it, expect } from 'vitest';
import { Mention } from './mention';

describe('Mention.serialize', () => {
    it('produces the exact token format', () => {
        expect(Mention.serialize(42, 'Jan Novák')).toBe('@[Jan Novák](user:42)');
    });
});

describe('Mention.parse', () => {
    it('returns a single text part when there are no mentions', () => {
        expect(Mention.parse('hello world')).toEqual([{ type: 'text', text: 'hello world' }]);
    });

    it('splits text around a mention', () => {
        expect(Mention.parse('cc @[Jan](user:42) please')).toEqual([
            { type: 'text', text: 'cc ' },
            { type: 'mention', idUser: 42, name: 'Jan' },
            { type: 'text', text: ' please' }
        ]);
    });

    it('handles multiple mentions and names with spaces', () => {
        expect(Mention.parse('@[Jan Novák](user:1)@[Eva](user:2)')).toEqual([
            { type: 'mention', idUser: 1, name: 'Jan Novák' },
            { type: 'mention', idUser: 2, name: 'Eva' }
        ]);
    });

    it('does NOT match a plain markdown link', () => {
        expect(Mention.parse('see [docs](http://x)')).toEqual([
            { type: 'text', text: 'see [docs](http://x)' }
        ]);
    });
});
