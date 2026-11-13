package scim

import "encoding/json"

const (
	SchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SchemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	BasePath           = "/api/scim/v2"
	ContentType        = "application/scim+json"
	maxPageSize        = 100
)

type Email struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type Name struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type Meta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location"`
}

type UserResource struct {
	Schemas     []string `json:"schemas"`
	Id          string   `json:"id"`
	ExternalId  string   `json:"externalId,omitempty"`
	UserName    string   `json:"userName"`
	Name        Name     `json:"name"`
	DisplayName string   `json:"displayName"`
	Emails      []Email  `json:"emails"`
	Active      bool     `json:"active"`
	Meta        Meta     `json:"meta"`
}

type ListResponse struct {
	Schemas      []string       `json:"schemas"`
	TotalResults int64          `json:"totalResults"`
	StartIndex   int64          `json:"startIndex"`
	ItemsPerPage int            `json:"itemsPerPage"`
	Resources    []UserResource `json:"Resources"`
}

type PatchRequest struct {
	Operations []PatchOperation `json:"Operations"`
}

type PatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type Account struct {
	IdUser     int64   `db:"id_user"`
	Name       string  `db:"name"`
	Email      string  `db:"email"`
	IsActive   bool    `db:"is_active"`
	ExternalId *string `db:"scim_external_id"`
}
