export enum SsoErrorCode {
    SsoUnavailable = 'sso_unavailable',
    InvalidState = 'invalid_state',
    IdpError = 'idp_error',
    Internal = 'internal',
    EmailMissing = 'email_missing',
    DomainNotAllowed = 'domain_not_allowed',
    EmailNotVerified = 'email_not_verified',
    EmailAmbiguous = 'email_ambiguous',
    AccountNotLinkable = 'account_not_linkable',
    AccountDisabled = 'account_disabled',
    AccountNotProvisioned = 'account_not_provisioned',
    SamlSubjectUnsupported = 'saml_subject_unsupported'
}
