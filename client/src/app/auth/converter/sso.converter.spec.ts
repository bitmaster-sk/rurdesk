import { SsoConverter } from './sso.converter';

describe('SsoConverter', () => {
    it('maps a known error code to its translation key', () => {
        expect(SsoConverter.toErrorKey('account_not_provisioned')).toBe(
            'LOGIN.SSO_ERROR.ACCOUNT_NOT_PROVISIONED'
        );
        expect(SsoConverter.toErrorKey('saml_subject_unsupported')).toBe(
            'LOGIN.SSO_ERROR.SAML_SUBJECT_UNSUPPORTED'
        );
    });

    it('maps an unknown error code to the generic key', () => {
        expect(SsoConverter.toErrorKey('<script>')).toBe('LOGIN.SSO_ERROR.UNKNOWN');
    });

    it('reads the code from the URL fragment', () => {
        expect(SsoConverter.toCode('code=abc-123')).toBe('abc-123');
        expect(SsoConverter.toCode('')).toBeNull();
        expect(SsoConverter.toCode(null)).toBeNull();
    });

    it('adds the extended flag to the start URL only when asked', () => {
        expect(SsoConverter.toStartUrl(false)).toBe('/api/public/sso/start');
        expect(SsoConverter.toStartUrl(true)).toBe('/api/public/sso/start?extended=true');
    });
});
