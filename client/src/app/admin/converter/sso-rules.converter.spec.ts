import { SsoRulesConverter } from './sso-rules.converter';

describe('SsoRulesConverter', () => {
    it('splits domains on new lines, commas and spaces', () => {
        expect(SsoRulesConverter.toDomains(' acme.test,\n b.acme.test  c.test \n\n')).toEqual([
            'acme.test',
            'b.acme.test',
            'c.test'
        ]);
        expect(SsoRulesConverter.toDomains('')).toEqual([]);
    });

    it('puts one domain on each line', () => {
        expect(SsoRulesConverter.toText(['acme.test', 'b.acme.test'])).toBe(
            'acme.test\nb.acme.test'
        );
    });
});
