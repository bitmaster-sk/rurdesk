package scim

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/email"
)

var workEmailPath = regexp.MustCompile(`^emails\[\s*type\s+eq\s+"work"\s*\]\.value$`)

type Changes struct {
	Active      *bool
	UserName    *string
	DisplayName *string
	Formatted   *string
	GivenName   *string
	FamilyName  *string
	ExternalId  *string
	Emails      []Email
	HasEmails   bool
	WorkEmail   *string
	// IsPartial marks a PATCH, which carries only the attributes that changed.
	IsPartial bool
}

func ParseResource(body []byte) (Changes, error) {
	var changes Changes
	return changes, changes.applyObject(body)
}

func ParsePatch(body []byte) (Changes, error) {
	var request PatchRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return Changes{}, invalidSyntax("the body is not a PatchOp")
	}
	changes := Changes{IsPartial: true}
	for _, operation := range request.Operations {
		switch strings.ToLower(operation.Op) {
		case "remove":
			continue
		case "add", "replace":
		default:
			return Changes{}, invalidSyntax("unsupported patch operation " + operation.Op)
		}
		var err error
		if operation.Path == "" {
			err = changes.applyObject(operation.Value)
		} else {
			err = changes.applyPath(operation.Path, operation.Value)
		}
		if err != nil {
			return Changes{}, err
		}
	}
	return changes, nil
}

func (c Changes) IsDeactivation() bool {
	return c.Active != nil && !*c.Active
}

func (c Changes) Email() (string, bool) {
	if c.HasEmails {
		if picked := pickEmail(c.Emails); picked != "" {
			return email.Normalize(picked), true
		}
	}
	for _, candidate := range []*string{c.WorkEmail, c.UserName} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			return email.Normalize(*candidate), true
		}
	}
	return "", false
}

func (c Changes) Name() (string, bool) {
	for _, candidate := range []*string{c.DisplayName, c.Formatted} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			return strings.TrimSpace(*candidate), true
		}
	}
	// Only the whole name is stored, so one part of a PATCH cannot be merged into it.
	if c.IsPartial && (c.GivenName == nil || c.FamilyName == nil) {
		return "", false
	}
	parts := make([]string, 0, 2)
	for _, part := range []*string{c.GivenName, c.FamilyName} {
		if part != nil && strings.TrimSpace(*part) != "" {
			parts = append(parts, strings.TrimSpace(*part))
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " "), true
}

func pickEmail(emails []Email) string {
	for _, candidate := range emails {
		if candidate.Primary && strings.TrimSpace(candidate.Value) != "" {
			return candidate.Value
		}
	}
	for _, candidate := range emails {
		if strings.EqualFold(candidate.Type, "work") && strings.TrimSpace(candidate.Value) != "" {
			return candidate.Value
		}
	}
	for _, candidate := range emails {
		if strings.TrimSpace(candidate.Value) != "" {
			return candidate.Value
		}
	}
	return ""
}

func (c *Changes) applyObject(raw json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return invalidSyntax("the value must be an object")
	}
	for key, value := range object {
		if err := c.applyPath(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (c *Changes) applyPath(rawPath string, value json.RawMessage) error {
	path := strings.ToLower(strings.TrimSpace(rawPath))
	path = strings.TrimPrefix(path, strings.ToLower(SchemaUser)+":")
	switch {
	case path == "active":
		active, err := parseBool(value)
		if err != nil {
			return err
		}
		c.Active = &active
	case path == "username":
		return setString(&c.UserName, value)
	case path == "displayname":
		return setString(&c.DisplayName, value)
	case path == "externalid":
		return setString(&c.ExternalId, value)
	case path == "name":
		return c.applyName(value)
	case path == "name.formatted":
		return setString(&c.Formatted, value)
	case path == "name.givenname":
		return setString(&c.GivenName, value)
	case path == "name.familyname":
		return setString(&c.FamilyName, value)
	case path == "emails":
		var emails []Email
		if err := json.Unmarshal(value, &emails); err != nil {
			return invalidValue("emails must be a list")
		}
		c.Emails, c.HasEmails = emails, true
	case workEmailPath.MatchString(path):
		return setString(&c.WorkEmail, value)
	}
	return nil
}

func (c *Changes) applyName(value json.RawMessage) error {
	var name map[string]json.RawMessage
	if err := json.Unmarshal(value, &name); err != nil {
		return invalidValue("name must be an object")
	}
	for key, part := range name {
		if err := c.applyPath("name."+key, part); err != nil {
			return err
		}
	}
	return nil
}

func setString(target **string, value json.RawMessage) error {
	var text *string
	if err := json.Unmarshal(value, &text); err != nil {
		return invalidValue("expected a string")
	}
	if text == nil {
		empty := ""
		text = &empty
	}
	*target = text
	return nil
}

func parseBool(value json.RawMessage) (bool, error) {
	var flag bool
	if err := json.Unmarshal(value, &flag); err == nil {
		return flag, nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		if parsed, err := strconv.ParseBool(strings.ToLower(text)); err == nil {
			return parsed, nil
		}
	}
	return false, invalidValue("active must be a boolean")
}
