package scim

import (
	"fmt"
	"net/http"
)

type Error struct {
	Status   int
	ScimType string
	Detail   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("scim %d %s: %s", e.Status, e.ScimType, e.Detail)
}

type ErrorBody struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

func (e *Error) Body() ErrorBody {
	return ErrorBody{Schemas: []string{SchemaError}, Status: fmt.Sprint(e.Status), ScimType: e.ScimType, Detail: e.Detail}
}

func invalidValue(detail string) *Error {
	return &Error{Status: http.StatusBadRequest, ScimType: "invalidValue", Detail: detail}
}

func invalidSyntax(detail string) *Error {
	return &Error{Status: http.StatusBadRequest, ScimType: "invalidSyntax", Detail: detail}
}

func invalidFilter(detail string) *Error {
	return &Error{Status: http.StatusBadRequest, ScimType: "invalidFilter", Detail: detail}
}

func uniqueness(detail string) *Error {
	return &Error{Status: http.StatusConflict, ScimType: "uniqueness", Detail: detail}
}

var (
	errNotFound     = &Error{Status: http.StatusNotFound, Detail: "user not found"}
	errUnauthorized = &Error{Status: http.StatusUnauthorized, Detail: "the SCIM token is missing, wrong or SCIM is off"}
	errForbidden    = &Error{Status: http.StatusForbidden, Detail: "the license does not include SSO"}
	errLastAdmin    = &Error{Status: http.StatusConflict, Detail: "cannot deactivate the last active admin"}
	errInternal     = &Error{Status: http.StatusInternalServerError, Detail: "internal error"}
)
