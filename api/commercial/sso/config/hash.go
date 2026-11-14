package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const fetchedMetadata = "fetched"

type hashInput struct {
	Protocol         string `json:"protocol"`
	Issuer           string `json:"issuer"`
	ClientId         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	HasGroupsScope   bool   `json:"hasGroupsScope"`
	EmailClaim       string `json:"emailClaim"`
	GroupsClaim      string `json:"groupsClaim"`
	MetadataUrl      string `json:"metadataUrl"`
	MetadataXml      string `json:"metadataXml"`
	NameAttribute    string `json:"nameAttribute"`
	SubjectAttribute string `json:"subjectAttribute"`
}

func Hash(key []byte, connection Connection) (string, error) {
	input := hashInput{
		Protocol:         connection.Protocol,
		Issuer:           connection.Issuer,
		ClientId:         connection.ClientId,
		ClientSecret:     connection.ClientSecret,
		HasGroupsScope:   connection.HasGroupsScope,
		EmailClaim:       connection.EmailClaim,
		GroupsClaim:      connection.GroupsClaim,
		MetadataUrl:      connection.MetadataUrl,
		MetadataXml:      connection.MetadataXml,
		NameAttribute:    connection.NameAttribute,
		SubjectAttribute: connection.SubjectAttribute,
	}
	// What a metadata URL serves is downloaded again on its own, so only the URL and having a copy count.
	if connection.MetadataUrl != "" {
		input.Issuer = ""
		if input.MetadataXml != "" {
			input.MetadataXml = fetchedMetadata
		}
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encoding sso connection: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func matchesHash(testedHash *string, hash string) bool {
	return testedHash != nil && hmac.Equal([]byte(*testedHash), []byte(hash))
}
