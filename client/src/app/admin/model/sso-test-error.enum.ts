export enum SsoTestError {
    IdpError = 'idp_error',
    TokenInvalid = 'token_invalid',
    SubjectMissing = 'subject_missing',
    SubjectUnsupported = 'subject_unsupported',
    DraftChanged = 'draft_changed',
    SsoUnavailable = 'sso_unavailable',
    Internal = 'internal'
}
