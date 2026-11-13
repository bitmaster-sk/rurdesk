package samlauth

import (
	"errors"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/email"
	"github.com/crewjam/saml"
)

const (
	nameIdTransient   = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	nameIdEmail       = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	nameIdUnspecified = "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
)

var (
	ErrSubjectMissing     = errors.New("the assertion has no subject")
	ErrSubjectUnsupported = errors.New("the assertion has a transient NameID, set a persistent NameID or a subject attribute")
)

func IdentityFrom(connection config.Connection, assertion *saml.Assertion) (identity.Identity, error) {
	attributes := attributesOf(assertion)
	nameId, nameIdFormat := "", ""
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		nameId, nameIdFormat = strings.TrimSpace(assertion.Subject.NameID.Value), assertion.Subject.NameID.Format
	}
	id := identity.Identity{Issuer: connection.Issuer}
	if connection.SubjectAttribute != "" {
		id.Subject = first(attributes, connection.SubjectAttribute)
	} else {
		if nameIdFormat == nameIdTransient {
			return identity.Identity{}, ErrSubjectUnsupported
		}
		id.Subject = nameId
		id.IsSubjectUnstable = nameIdFormat == nameIdEmail || nameIdFormat == nameIdUnspecified || nameIdFormat == ""
	}
	if id.Subject == "" {
		return identity.Identity{}, ErrSubjectMissing
	}
	id.Email = email.Normalize(first(attributes, connection.EmailClaim))
	if id.Email == "" && nameIdFormat == nameIdEmail {
		id.Email = email.Normalize(nameId)
	}
	id.Name = first(attributes, connection.NameAttribute)
	id.Groups, id.HasGroupsClaim = attributes[connection.GroupsClaim]
	return id, nil
}

func attributesOf(assertion *saml.Assertion) map[string][]string {
	attributes := map[string][]string{}
	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			values := make([]string, 0, len(attribute.Values))
			for _, value := range attribute.Values {
				if trimmed := strings.TrimSpace(value.Value); trimmed != "" {
					values = append(values, trimmed)
				}
			}
			for _, key := range []string{attribute.Name, attribute.FriendlyName} {
				if key != "" {
					attributes[key] = append(attributes[key], values...)
				}
			}
		}
	}
	return attributes
}

func first(attributes map[string][]string, name string) string {
	if values := attributes[name]; len(values) > 0 {
		return values[0]
	}
	return ""
}
