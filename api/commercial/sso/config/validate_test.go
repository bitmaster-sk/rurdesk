package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIssuer(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
		err      error
	}{
		{name: "https", raw: " https://idp.example.com/realms/acme ", expected: "https://idp.example.com/realms/acme"},
		{name: "keeps trailing slash", raw: "https://acme.okta.com/", expected: "https://acme.okta.com/"},
		{name: "loopback http", raw: "http://127.0.0.1:5556", expected: "http://127.0.0.1:5556"},
		{name: "localhost http", raw: "http://localhost:8080/realms/dev", expected: "http://localhost:8080/realms/dev"},
		{name: "single tenant entra", raw: "https://login.microsoftonline.com/0b6c4f2e-1111-2222-3333-444455556666/v2.0", expected: "https://login.microsoftonline.com/0b6c4f2e-1111-2222-3333-444455556666/v2.0"},
		{name: "public http", raw: "http://idp.example.com", err: ErrIssuerInvalid},
		{name: "no host", raw: "https://", err: ErrIssuerInvalid},
		{name: "relative", raw: "idp.example.com", err: ErrIssuerInvalid},
		{name: "query", raw: "https://idp.example.com?x=1", err: ErrIssuerInvalid},
		{name: "fragment", raw: "https://idp.example.com#x", err: ErrIssuerInvalid},
		{name: "userinfo", raw: "https://user@idp.example.com", err: ErrIssuerInvalid},
		{name: "entra common", raw: "https://login.microsoftonline.com/common/v2.0", err: ErrIssuerMultiTenant},
		{name: "entra organizations", raw: "https://login.microsoftonline.com/organizations/v2.0", err: ErrIssuerMultiTenant},
		{name: "entra consumers upper case", raw: "https://LOGIN.microsoftonline.com/Consumers/v2.0", err: ErrIssuerMultiTenant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer, err := ValidateIssuer(tt.raw)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, issuer)
		})
	}
}

func TestValidatePublicBaseUrl(t *testing.T) {
	tests := []struct {
		name    string
		baseUrl string
		isValid bool
	}{
		{name: "https", baseUrl: "https://tracker.example.com", isValid: true},
		{name: "https with port", baseUrl: "https://tracker.example.com:8443", isValid: true},
		{name: "localhost http", baseUrl: "http://localhost", isValid: true},
		{name: "empty"},
		{name: "public http", baseUrl: "http://tracker.example.com"},
		{name: "path", baseUrl: "https://tracker.example.com/app"},
		{name: "no scheme", baseUrl: "tracker.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePublicBaseUrl(tt.baseUrl)
			if tt.isValid {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrPublicUrlInvalid)
		})
	}
}

func TestNormalizeDomains(t *testing.T) {
	domains, err := NormalizeDomains([]string{" Acme.TEST ", "acme.test", "", "b.acme.test"})
	require.NoError(t, err)
	assert.Equal(t, []string{"acme.test", "b.acme.test"}, domains)

	empty, err := NormalizeDomains(nil)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	for _, invalid := range []string{"acme", "@acme.test", "acme..test", "https://acme.test", "-acme.test"} {
		_, err := NormalizeDomains([]string{invalid})
		assert.ErrorIs(t, err, ErrDomainInvalid, invalid)
	}
}
