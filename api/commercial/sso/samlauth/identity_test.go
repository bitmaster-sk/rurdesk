package samlauth

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/crewjam/saml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var connection = config.Connection{
	Protocol:      config.ProtocolSaml,
	Issuer:        "https://idp.acme.test/metadata",
	EmailClaim:    "email",
	GroupsClaim:   "groups",
	NameAttribute: "displayName",
}

func assertion(nameIdFormat, nameId string, attributes ...saml.Attribute) *saml.Assertion {
	return &saml.Assertion{
		Subject:             &saml.Subject{NameID: &saml.NameID{Format: nameIdFormat, Value: nameId}},
		AttributeStatements: []saml.AttributeStatement{{Attributes: attributes}},
	}
}

func attribute(name, friendlyName string, values ...string) saml.Attribute {
	attribute := saml.Attribute{Name: name, FriendlyName: friendlyName}
	for _, value := range values {
		attribute.Values = append(attribute.Values, saml.AttributeValue{Value: value})
	}
	return attribute
}

func TestIdentityFrom_MapsTheAttributes(t *testing.T) {
	id, err := IdentityFrom(connection, assertion("urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", "abc-123",
		attribute("urn:oid:0.9.2342.19200300.100.1.3", "email", "Jana@Acme.test"),
		attribute("displayName", "", "Jana Nováková"),
		attribute("groups", "", "dev", "ops"),
	))

	require.NoError(t, err)
	assert.Equal(t, "https://idp.acme.test/metadata", id.Issuer)
	assert.Equal(t, "abc-123", id.Subject)
	assert.Equal(t, "jana@acme.test", id.Email, "the friendly name matches too")
	assert.Nil(t, id.EmailVerified)
	assert.Equal(t, "Jana Nováková", id.Name)
	assert.Equal(t, []string{"dev", "ops"}, id.Groups)
	assert.True(t, id.HasGroupsClaim)
	assert.False(t, id.IsSubjectUnstable)
}

func TestIdentityFrom_SubjectRules(t *testing.T) {
	_, err := IdentityFrom(connection, assertion("urn:oasis:names:tc:SAML:2.0:nameid-format:transient", "tmp-1"))
	assert.ErrorIs(t, err, ErrSubjectUnsupported)

	_, err = IdentityFrom(connection, assertion("urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", " "))
	assert.ErrorIs(t, err, ErrSubjectMissing)

	byEmail, err := IdentityFrom(connection, assertion("urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", "Jana@Acme.test"))
	require.NoError(t, err)
	assert.True(t, byEmail.IsSubjectUnstable)
	assert.Equal(t, "jana@acme.test", byEmail.Email, "an e-mail NameID fills a missing e-mail attribute")
	assert.False(t, byEmail.HasGroupsClaim)

	withAttribute := connection
	withAttribute.SubjectAttribute = "objectId"
	fromAttribute, err := IdentityFrom(withAttribute, assertion("urn:oasis:names:tc:SAML:2.0:nameid-format:transient", "tmp-1",
		attribute("objectId", "", "oid-7")))
	require.NoError(t, err)
	assert.Equal(t, "oid-7", fromAttribute.Subject, "a subject attribute makes a transient NameID fine")
	assert.False(t, fromAttribute.IsSubjectUnstable)

	_, err = IdentityFrom(withAttribute, assertion("urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", "abc"))
	assert.ErrorIs(t, err, ErrSubjectMissing, "a configured subject attribute must be present")
}
