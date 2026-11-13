package scim

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	schemaServiceProviderConfig = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	schemaResourceType          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	schemaSchema                = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

type supported struct {
	Supported bool `json:"supported"`
}

type filterSupport struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type bulkSupport struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

type authenticationScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type attribute struct {
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	MultiValued   bool        `json:"multiValued"`
	Required      bool        `json:"required"`
	CaseExact     bool        `json:"caseExact"`
	Mutability    string      `json:"mutability"`
	Returned      string      `json:"returned"`
	Uniqueness    string      `json:"uniqueness"`
	SubAttributes []attribute `json:"subAttributes,omitempty"`
}

func stringAttribute(name string, isRequired bool, uniqueness string) attribute {
	return attribute{Name: name, Type: "string", Required: isRequired, Mutability: "readWrite", Returned: "default", Uniqueness: uniqueness}
}

func serviceProviderConfig(c *gin.Context) {
	respond(c, http.StatusOK, gin.H{
		"schemas":        []string{schemaServiceProviderConfig},
		"patch":          supported{Supported: true},
		"bulk":           bulkSupport{},
		"filter":         filterSupport{Supported: true, MaxResults: maxPageSize},
		"changePassword": supported{},
		"sort":           supported{},
		"etag":           supported{},
		"authenticationSchemes": []authenticationScheme{{
			Type:        "oauthbearertoken",
			Name:        "Bearer token",
			Description: "The SCIM token from Admin → Settings → Sign-in and SSO",
		}},
		"meta": gin.H{"resourceType": "ServiceProviderConfig", "location": BasePath + "/ServiceProviderConfig"},
	})
}

func resourceTypes(c *gin.Context) {
	respond(c, http.StatusOK, gin.H{
		"schemas":      []string{SchemaListResponse},
		"totalResults": 1,
		"startIndex":   1,
		"itemsPerPage": 1,
		"Resources": []gin.H{{
			"schemas":  []string{schemaResourceType},
			"id":       "User",
			"name":     "User",
			"endpoint": "/Users",
			"schema":   SchemaUser,
			"meta":     gin.H{"resourceType": "ResourceType", "location": BasePath + "/ResourceTypes/User"},
		}},
	})
}

func schemas(c *gin.Context) {
	emails := attribute{Name: "emails", Type: "complex", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none",
		SubAttributes: []attribute{stringAttribute("value", false, "none"), stringAttribute("type", false, "none"),
			{Name: "primary", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}}}
	name := attribute{Name: "name", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none",
		SubAttributes: []attribute{stringAttribute("formatted", false, "none"), stringAttribute("givenName", false, "none"), stringAttribute("familyName", false, "none")}}
	respond(c, http.StatusOK, gin.H{
		"schemas":      []string{SchemaListResponse},
		"totalResults": 1,
		"startIndex":   1,
		"itemsPerPage": 1,
		"Resources": []gin.H{{
			"schemas":     []string{schemaSchema},
			"id":          SchemaUser,
			"name":        "User",
			"description": "User account",
			"attributes": []attribute{
				stringAttribute("userName", true, "server"),
				stringAttribute("displayName", false, "none"),
				stringAttribute("externalId", false, "server"),
				name,
				emails,
				{Name: "active", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
			},
			"meta": gin.H{"resourceType": "Schema", "location": BasePath + "/Schemas/" + SchemaUser},
		}},
	})
}
