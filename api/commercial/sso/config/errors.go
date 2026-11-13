package config

import "github.com/bitmaster-sk/rurdesk/api/internal/errs"

var (
	ErrKeyMissing          = errs.NewErr("SSO_KEY_MISSING", "SSO_ENCRYPTION_KEY must be 32 bytes encoded in base64", "error.sso_key_missing")
	ErrSecretUndecryptable = errs.NewErr("SSO_SECRET_UNDECRYPTABLE", "the stored client secret cannot be decrypted with SSO_ENCRYPTION_KEY", "error.sso_secret_undecryptable")
	ErrSecretRequired      = errs.NewErr("SSO_SECRET_REQUIRED", "client secret is required", "error.sso_secret_required")
	ErrNotConfigured       = errs.NewErr("SSO_NOT_CONFIGURED", "no SSO connection is saved", "error.sso_not_configured")
	ErrNotTested           = errs.NewErr("SSO_NOT_TESTED", "the draft has no successful connection test", "error.sso_not_tested")
	ErrDraftChanged        = errs.NewErr("SSO_DRAFT_CHANGED", "the draft changed during the connection test", "error.sso_draft_changed")
	ErrPublicUrlInvalid    = errs.NewErr("SSO_PUBLIC_URL_INVALID", "PUBLIC_BASE_URL must be an https URL without a path", "error.sso_public_url_invalid")
	ErrIssuerInvalid       = errs.NewErr("SSO_ISSUER_INVALID", "issuer must be an https URL without query, fragment or user info", "error.sso_issuer_invalid")
	ErrIssuerMultiTenant   = errs.NewErr("SSO_ISSUER_MULTI_TENANT", "multi-tenant Microsoft issuers are not supported, use the tenant issuer", "error.sso_issuer_multi_tenant")
	ErrDomainInvalid       = errs.NewErr("SSO_DOMAIN_INVALID", "allowed domain is not a valid domain name", "error.sso_domain_invalid")
	ErrClientIdRequired    = errs.NewErr("SSO_CLIENT_ID_REQUIRED", "issuer and client ID are required", "error.sso_client_id_required")
	ErrMetadataRequired    = errs.NewErr("SSO_METADATA_REQUIRED", "a metadata URL or metadata XML of the identity provider is required", "error.sso_metadata_required")
	ErrMetadataAmbiguous   = errs.NewErr("SSO_METADATA_AMBIGUOUS", "fill in either the metadata URL or the metadata XML, not both", "error.sso_metadata_ambiguous")
	ErrMetadataInvalid     = errs.NewErr("SSO_METADATA_INVALID", "the identity provider metadata is not valid SAML metadata of an identity provider", "error.sso_metadata_invalid")
	ErrMetadataUrlInvalid  = errs.NewErr("SSO_METADATA_URL_INVALID", "the metadata URL must be an https URL", "error.sso_metadata_url_invalid")
	ErrSpKeyMissing        = errs.NewErr("SSO_SP_KEY_MISSING", "no SAML service provider certificate exists yet", "error.sso_sp_key_missing")
	ErrSpKeyUndecryptable  = errs.NewErr("SSO_SP_KEY_UNDECRYPTABLE", "the SAML service provider key cannot be decrypted with SSO_ENCRYPTION_KEY", "error.sso_sp_key_undecryptable")
	ErrSpKeyReadable       = errs.NewErr("SSO_SP_KEY_READABLE", "the SAML service provider key still works, it is only replaced when it cannot be decrypted", "error.sso_sp_key_readable")
)
