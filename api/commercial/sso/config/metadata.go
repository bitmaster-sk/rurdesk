package config

import (
	"net/url"
	"strings"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

func ParseIdpMetadata(raw string) (*saml.EntityDescriptor, error) {
	descriptor, err := samlsp.ParseMetadata([]byte(raw))
	if err != nil || descriptor.EntityID == "" || len(descriptor.IDPSSODescriptors) == 0 {
		return nil, ErrMetadataInvalid
	}
	return descriptor, nil
}

func ValidateMetadataUrl(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || !isSecureOrLoopback(parsed) {
		return "", ErrMetadataUrlInvalid
	}
	return trimmed, nil
}
