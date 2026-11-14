import { SsoTestAction } from '../model/sso-test-action.enum';
import { SsoTestResultConverter } from './sso-test-result.converter';

describe('SsoTestResultConverter', () => {
    it('maps error codes to keys with a fallback', () => {
        expect(SsoTestResultConverter.toErrorKey('draft_changed')).toBe(
            'ADMIN.SSO.TEST_ERROR.DRAFT_CHANGED'
        );
        expect(SsoTestResultConverter.toErrorKey('subject_unsupported')).toBe(
            'ADMIN.SSO.TEST_ERROR.SUBJECT_UNSUPPORTED'
        );
        expect(SsoTestResultConverter.toErrorKey('nope')).toBe('ADMIN.SSO.TEST_ERROR.UNKNOWN');
        expect(SsoTestResultConverter.toErrorKey(undefined)).toBe('ADMIN.SSO.TEST_ERROR.UNKNOWN');
    });

    it('maps warnings to keys with a fallback', () => {
        expect(
            SsoTestResultConverter.toWarningKeys([
                'groups_overage',
                'saml_subject_unstable',
                'nope'
            ])
        ).toEqual([
            'ADMIN.SSO.WARNING.GROUPS_OVERAGE',
            'ADMIN.SSO.WARNING.SAML_SUBJECT_UNSTABLE',
            'ADMIN.SSO.WARNING.UNKNOWN'
        ]);
    });

    it('maps the predicted outcome and reuses the login reason texts', () => {
        expect(SsoTestResultConverter.toOutcomeKey(SsoTestAction.Create)).toBe(
            'ADMIN.SSO.OUTCOME.CREATE'
        );
        expect(SsoTestResultConverter.toOutcomeKey(undefined)).toBe('ADMIN.SSO.OUTCOME.UNKNOWN');
        expect(
            SsoTestResultConverter.toReasonKey({
                action: SsoTestAction.Reject,
                reason: 'domain_not_allowed'
            })
        ).toBe('LOGIN.SSO_ERROR.DOMAIN_NOT_ALLOWED');
        expect(SsoTestResultConverter.toReasonKey({ action: SsoTestAction.Link })).toBeNull();
    });
});
