package config

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/breakglass"
)

const (
	ProtocolOidc            = "oidc"
	ProtocolSaml            = "saml"
	DefaultEmailClaim       = "email"
	DefaultGroupsClaim      = "groups"
	DefaultNameAttribute    = "name"
	CallbackPath            = "/api/public/sso/oidc/callback"
	AcsPath                 = "/api/public/sso/saml/acs"
	SpMetadataPath          = "/api/public/sso/saml/metadata"
	MetadataRefreshInterval = 24 * time.Hour

	kindDraft  = "draft"
	kindActive = "active"
)

type Connection struct {
	Protocol          string
	Issuer            string
	ClientId          string
	ClientSecret      string
	HasGroupsScope    bool
	EmailClaim        string
	GroupsClaim       string
	MetadataUrl       string
	MetadataXml       string
	NameAttribute     string
	SubjectAttribute  string
	MetadataFetchedAt *time.Time
}

func (c Connection) NeedsMetadataRefresh(now time.Time) bool {
	return c.Protocol == ProtocolSaml && c.MetadataUrl != "" &&
		(c.MetadataFetchedAt == nil || now.Sub(*c.MetadataFetchedAt) > MetadataRefreshInterval)
}

type Settings struct {
	IsEnabled      bool     `db:"is_enabled"`
	AllowedDomains []string `db:"allowed_domains"`
	IsAutoCreate   bool     `db:"is_auto_create"`
	AdminGroup     string   `db:"admin_group"`
	IsEnforced     bool     `db:"is_enforced"`
	ActiveProtocol string   `db:"active_protocol"`
}

type storedConnection struct {
	Protocol          string     `db:"protocol"`
	Issuer            string     `db:"issuer"`
	ClientId          string     `db:"client_id"`
	ClientSecretEnc   []byte     `db:"client_secret_enc"`
	ClientSecretNonce []byte     `db:"client_secret_nonce"`
	HasGroupsScope    bool       `db:"scopes_groups"`
	EmailClaim        string     `db:"email_claim"`
	GroupsClaim       string     `db:"groups_claim"`
	MetadataUrl       string     `db:"idp_metadata_url"`
	MetadataXml       string     `db:"idp_metadata_xml"`
	MetadataFetchedAt *time.Time `db:"idp_metadata_fetched_at"`
	NameAttribute     string     `db:"name_attribute"`
	SubjectAttribute  string     `db:"subject_attribute"`
	TestedHash        *string    `db:"tested_hash"`
	UpdateAt          time.Time  `db:"update_at"`
}

func (s *storedConnection) connection(clientSecret string) Connection {
	return Connection{
		Protocol:          s.Protocol,
		Issuer:            s.Issuer,
		ClientId:          s.ClientId,
		ClientSecret:      clientSecret,
		HasGroupsScope:    s.HasGroupsScope,
		EmailClaim:        s.EmailClaim,
		GroupsClaim:       s.GroupsClaim,
		MetadataUrl:       s.MetadataUrl,
		MetadataXml:       s.MetadataXml,
		NameAttribute:     s.NameAttribute,
		SubjectAttribute:  s.SubjectAttribute,
		MetadataFetchedAt: s.MetadataFetchedAt,
	}
}

func (s *storedConnection) response() *ConnectionRes {
	return &ConnectionRes{
		Protocol:          s.Protocol,
		Issuer:            s.Issuer,
		ClientId:          s.ClientId,
		HasClientSecret:   len(s.ClientSecretEnc) > 0,
		HasGroupsScope:    s.HasGroupsScope,
		EmailClaim:        s.EmailClaim,
		GroupsClaim:       s.GroupsClaim,
		MetadataUrl:       s.MetadataUrl,
		HasMetadata:       s.MetadataXml != "",
		MetadataFetchedAt: s.MetadataFetchedAt,
		NameAttribute:     s.NameAttribute,
		SubjectAttribute:  s.SubjectAttribute,
		UpdateAt:          s.UpdateAt,
	}
}

type SaveDraftReq struct {
	Protocol         string `json:"protocol" binding:"required,oneof=oidc saml"`
	Issuer           string `json:"issuer" binding:"max=500"`
	ClientId         string `json:"clientId" binding:"max=500"`
	ClientSecret     string `json:"clientSecret" binding:"max=2000"`
	HasGroupsScope   bool   `json:"hasGroupsScope"`
	EmailClaim       string `json:"emailClaim" binding:"max=300"`
	GroupsClaim      string `json:"groupsClaim" binding:"max=300"`
	MetadataUrl      string `json:"metadataUrl" binding:"max=2000"`
	MetadataXml      string `json:"metadataXml" binding:"max=500000"`
	NameAttribute    string `json:"nameAttribute" binding:"max=300"`
	SubjectAttribute string `json:"subjectAttribute" binding:"max=300"`
}

type UpdateRulesReq struct {
	AllowedDomains []string `json:"allowedDomains" binding:"max=100,dive,max=253"`
	IsAutoCreate   bool     `json:"isAutoCreate"`
	AdminGroup     string   `json:"adminGroup" binding:"max=500"`
}

type ConnectionRes struct {
	Protocol          string     `json:"protocol"`
	Issuer            string     `json:"issuer"`
	ClientId          string     `json:"clientId"`
	HasClientSecret   bool       `json:"hasClientSecret"`
	HasGroupsScope    bool       `json:"hasGroupsScope"`
	EmailClaim        string     `json:"emailClaim"`
	GroupsClaim       string     `json:"groupsClaim"`
	MetadataUrl       string     `json:"metadataUrl"`
	HasMetadata       bool       `json:"hasMetadata"`
	MetadataFetchedAt *time.Time `json:"metadataFetchedAt"`
	NameAttribute     string     `json:"nameAttribute"`
	SubjectAttribute  string     `json:"subjectAttribute"`
	UpdateAt          time.Time  `json:"updateAt"`
}

type RulesRes struct {
	AllowedDomains []string `json:"allowedDomains"`
	IsAutoCreate   bool     `json:"isAutoCreate"`
	AdminGroup     string   `json:"adminGroup"`
}

type SetEnforcedReq struct {
	IsEnforced *bool `json:"isEnforced" binding:"required"`
}

type StateRes struct {
	IsLicensed            bool                 `json:"isLicensed"`
	HasEncryptionKey      bool                 `json:"hasEncryptionKey"`
	IsSecretUndecryptable bool                 `json:"isSecretUndecryptable"`
	IsSpKeyUndecryptable  bool                 `json:"isSpKeyUndecryptable"`
	PublicBaseUrl         string               `json:"publicBaseUrl"`
	IsPublicBaseUrlValid  bool                 `json:"isPublicBaseUrlValid"`
	RedirectUri           string               `json:"redirectUri"`
	AcsUrl                string               `json:"acsUrl"`
	SpEntityId            string               `json:"spEntityId"`
	Draft                 *ConnectionRes       `json:"draft"`
	Active                *ConnectionRes       `json:"active"`
	IsDraftTested         bool                 `json:"isDraftTested"`
	IsDraftActive         bool                 `json:"isDraftActive"`
	IsEnabled             bool                 `json:"isEnabled"`
	Rules                 RulesRes             `json:"rules"`
	IsEnforced            bool                 `json:"isEnforced"`
	BreakGlassAccounts    []breakglass.Account `json:"breakGlassAccounts"`
}
