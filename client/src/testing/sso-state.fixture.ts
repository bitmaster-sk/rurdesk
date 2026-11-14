import { SsoProtocol } from 'src/app/admin/model/sso-protocol.enum';
import { SsoState } from 'src/app/admin/model/admin-sso.model';

export function ssoState(changes: Partial<SsoState> = {}): SsoState {
    return {
        isLicensed: true,
        hasEncryptionKey: true,
        isSecretUndecryptable: false,
        isSpKeyUndecryptable: false,
        publicBaseUrl: 'https://tracker.acme.test',
        isPublicBaseUrlValid: true,
        redirectUri: 'https://tracker.acme.test/api/public/sso/oidc/callback',
        acsUrl: 'https://tracker.acme.test/api/public/sso/saml/acs',
        spEntityId: 'https://tracker.acme.test/api/public/sso/saml/metadata',
        draft: {
            protocol: SsoProtocol.Oidc,
            issuer: 'https://idp.acme.test',
            clientId: 'rurdesk',
            hasClientSecret: true,
            hasGroupsScope: false,
            emailClaim: 'email',
            groupsClaim: 'groups',
            metadataUrl: '',
            hasMetadata: false,
            metadataFetchedAt: null,
            nameAttribute: '',
            subjectAttribute: '',
            updateAt: '2026-11-10T10:00:00Z'
        },
        active: null,
        isDraftTested: false,
        isDraftActive: false,
        isEnabled: false,
        rules: { allowedDomains: ['acme.test'], isAutoCreate: false, adminGroup: '' },
        isEnforced: false,
        breakGlassAccounts: [],
        scim: {
            isEnabled: false,
            hasToken: false,
            tokenCreateAt: null,
            lastUsedAt: null,
            baseUrl: 'https://tracker.acme.test/api/scim/v2'
        },
        ...changes
    };
}
