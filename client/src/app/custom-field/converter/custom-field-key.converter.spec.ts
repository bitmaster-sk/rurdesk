import { CustomFieldKeyConverter } from './custom-field-key.converter';

describe('CustomFieldKeyConverter', () => {
    it('lowercases and joins words with an underscore', () => {
        expect(CustomFieldKeyConverter.toKey('Delivery Date')).toBe('delivery_date');
    });

    it('strips diacritics instead of keeping a non-ASCII key', () => {
        expect(CustomFieldKeyConverter.toKey('Termín dodania')).toBe('termin_dodania');
        expect(CustomFieldKeyConverter.toKey('Počet ľudí')).toBe('pocet_ludi');
    });

    it('gives the same key for both Unicode spellings of an accented name', () => {
        expect(CustomFieldKeyConverter.toKey('Termín')).toBe(
            CustomFieldKeyConverter.toKey('Termín'.normalize('NFD'))
        );
    });

    it('collapses punctuation and repeated separators', () => {
        expect(CustomFieldKeyConverter.toKey('Cena (bez DPH) — 2026')).toBe('cena_bez_dph_2026');
    });

    it('trims separators off both ends', () => {
        expect(CustomFieldKeyConverter.toKey('  ...Owner!  ')).toBe('owner');
    });

    it('keeps a leading digit rather than losing part of the name', () => {
        expect(CustomFieldKeyConverter.toKey('2026 plán')).toBe('2026_plan');
    });

    it('caps the key at the length the server accepts', () => {
        const key = CustomFieldKeyConverter.toKey('a'.repeat(60));

        expect(key).toHaveLength(CustomFieldKeyConverter.maxLength);
    });

    it('does not end a truncated key with a separator', () => {
        const key = CustomFieldKeyConverter.toKey(`${'a'.repeat(39)} tail`);

        expect(key).toBe('a'.repeat(39));
    });

    it('returns nothing when the name has no Latin letters to work with', () => {
        expect(CustomFieldKeyConverter.toKey('客户编号')).toBe('');
        expect(CustomFieldKeyConverter.toKey('???')).toBe('');
    });

    it('produces keys its own pattern accepts', () => {
        for (const name of ['Termín dodania', '2026 plán', 'Cena (bez DPH)']) {
            expect(CustomFieldKeyConverter.pattern.test(CustomFieldKeyConverter.toKey(name))).toBe(
                true
            );
        }
    });
});
