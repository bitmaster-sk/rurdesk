import { WikiSlugConverter } from './wiki-slug.converter';

describe('WikiSlugConverter', () => {
    it('produces the same slugs as the server', () => {
        expect(WikiSlugConverter.toSlug('Konvencie backendu')).toBe('konvencie-backendu');
        expect(WikiSlugConverter.toSlug('Prehľad systému')).toBe('prehlad-systemu');
        expect(WikiSlugConverter.toSlug('  ADR-004: Branch per run!  ')).toBe(
            'adr-004-branch-per-run'
        );
        expect(WikiSlugConverter.toSlug('Ťažké ŽLUŤOUČKÉ kôň')).toBe('tazke-zlutoucke-kon');
        expect(WikiSlugConverter.toSlug('Release 2.0 / notes & roadmap')).toBe(
            'release-2-0-notes-roadmap'
        );
    });

    it('returns an empty slug when nothing usable is left', () => {
        expect(WikiSlugConverter.toSlug('---')).toBe('');
    });

    it('caps the length without a trailing dash', () => {
        const slug = WikiSlugConverter.toSlug('ab '.repeat(100));
        expect(slug.length).toBeLessThanOrEqual(WikiSlugConverter.maxLength);
        expect(slug.endsWith('-')).toBe(false);
    });
});
