package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash_IsStableAndChangesWithEveryField(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	base := Connection{
		Protocol:     ProtocolOidc,
		Issuer:       "https://idp.example.com",
		ClientId:     "rurdesk",
		ClientSecret: "s3cret",
		EmailClaim:   DefaultEmailClaim,
		GroupsClaim:  DefaultGroupsClaim,
	}
	baseHash := mustHash(t, key, base)
	assert.Equal(t, baseHash, mustHash(t, key, base))

	changes := map[string]func(*Connection){
		"protocol":     func(c *Connection) { c.Protocol = "saml" },
		"issuer":       func(c *Connection) { c.Issuer = "https://other.example.com" },
		"client id":    func(c *Connection) { c.ClientId = "other" },
		"secret":       func(c *Connection) { c.ClientSecret = "other" },
		"groups scope": func(c *Connection) { c.HasGroupsScope = true },
		"email claim":  func(c *Connection) { c.EmailClaim = "preferred_username" },
		"groups claim": func(c *Connection) { c.GroupsClaim = "roles" },
		"metadata url": func(c *Connection) { c.MetadataUrl = "https://idp.example.com/metadata" },
		"metadata xml": func(c *Connection) { c.MetadataXml = "<EntityDescriptor/>" },
		"name":         func(c *Connection) { c.NameAttribute = "displayName" },
		"subject":      func(c *Connection) { c.SubjectAttribute = "objectId" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			changed := base
			change(&changed)
			assert.NotEqual(t, baseHash, mustHash(t, key, changed))
		})
	}

	assert.NotEqual(t, baseHash, mustHash(t, bytes.Repeat([]byte{8}, 32), base))

	refreshed := base
	refreshedAt := time.Now()
	refreshed.MetadataFetchedAt = &refreshedAt
	assert.Equal(t, baseHash, mustHash(t, key, refreshed), "a refresh of unchanged metadata keeps the test valid")
}

func TestHash_IgnoresMetadataFetchedFromTheUrl(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	fetched := Connection{
		Protocol:    ProtocolSaml,
		Issuer:      "https://idp.example.com/entity",
		MetadataUrl: "https://idp.example.com/metadata",
		MetadataXml: "<EntityDescriptor/>",
	}
	fetchedHash := mustHash(t, key, fetched)

	refreshed := fetched
	refreshed.Issuer, refreshed.MetadataXml = "https://idp.example.com/renamed", "<EntityDescriptor validUntil=\"later\"/>"
	assert.Equal(t, fetchedHash, mustHash(t, key, refreshed), "a download from the same URL is not an edit")

	notFetched := fetched
	notFetched.Issuer, notFetched.MetadataXml = "", ""
	assert.NotEqual(t, fetchedHash, mustHash(t, key, notFetched), "a test of downloaded metadata does not cover a draft without a copy")

	moved := fetched
	moved.MetadataUrl = "https://other.example.com/metadata"
	assert.NotEqual(t, fetchedHash, mustHash(t, key, moved))

	pasted := fetched
	pasted.MetadataUrl = ""
	edited := pasted
	edited.MetadataXml = "<EntityDescriptor validUntil=\"later\"/>"
	assert.NotEqual(t, mustHash(t, key, pasted), mustHash(t, key, edited), "pasted metadata is part of what was tested")
}

func mustHash(t *testing.T, key []byte, connection Connection) string {
	t.Helper()
	hash, err := Hash(key, connection)
	require.NoError(t, err)
	return hash
}
