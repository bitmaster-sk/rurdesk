package scim

import (
	"strconv"
	"strings"
)

const (
	AttributeUserName   = "userName"
	AttributeExternalId = "externalId"
)

type Filter struct {
	Attribute string
	Value     string
}

func ParseFilter(raw string) (*Filter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	attribute, rest, _ := strings.Cut(raw, " ")
	operator, quoted, _ := strings.Cut(strings.TrimSpace(rest), " ")
	if !strings.EqualFold(operator, "eq") {
		return nil, invalidFilter("only the eq operator is supported")
	}
	value, err := strconv.Unquote(strings.TrimSpace(quoted))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(quoted), `"`) {
		return nil, invalidFilter("the filter value must be a quoted string")
	}
	switch {
	case strings.EqualFold(attribute, AttributeUserName):
		return &Filter{Attribute: AttributeUserName, Value: value}, nil
	case strings.EqualFold(attribute, AttributeExternalId):
		return &Filter{Attribute: AttributeExternalId, Value: value}, nil
	}
	return nil, invalidFilter("only userName and externalId can be filtered")
}

type Page struct {
	StartIndex int64
	Count      int
}

func ParsePage(rawStart, rawCount string) Page {
	page := Page{StartIndex: 1, Count: maxPageSize}
	if start, err := strconv.ParseInt(rawStart, 10, 64); err == nil && start > 1 {
		page.StartIndex = start
	}
	if count, err := strconv.Atoi(rawCount); err == nil && count >= 0 && count < maxPageSize {
		page.Count = count
	}
	return page
}
