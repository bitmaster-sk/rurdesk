package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type CustomFieldValueService struct {
	customFieldRepo *repository.CustomFieldRepository
	valueRepo       *repository.CustomFieldValueRepository
}

func NewCustomFieldValueService(
	customFieldRepo *repository.CustomFieldRepository,
	valueRepo *repository.CustomFieldValueRepository,
) *CustomFieldValueService {
	return &CustomFieldValueService{customFieldRepo: customFieldRepo, valueRepo: valueRepo}
}

// A key absent from the map is left untouched; a key mapped to JSON null deletes
// the row. On create the required check must run even for an empty map, or a caller
// omitting customFields walks past it.
func (s *CustomFieldValueService) Apply(
	ctx context.Context,
	idProject, idIssue int64,
	raw map[string]json.RawMessage,
	isCreate bool,
) error {
	if len(raw) == 0 && !isCreate {
		return nil
	}
	fields, err := s.customFieldRepo.LoadCustomFieldsByProject(ctx, idProject)
	if err != nil {
		return err
	}
	byKey := make(map[string]*model.CustomField, len(fields))
	for _, field := range fields {
		byKey[field.Key] = field
	}
	if isCreate {
		raw = withDefaults(fields, raw)
	}

	upserts := make([]model.IssueCustomValue, 0, len(raw))
	deletes := make([]int64, 0, len(raw))

	for key, rawValue := range raw {
		field, known := byKey[key]
		if !known {
			return errs.ErrCustomFieldUnknownKey.WithMessage(fmt.Sprintf("unknown custom field %q", key))
		}
		if field.IsArchived() {
			return errs.ErrCustomFieldArchived.WithMessage(fmt.Sprintf("field %q is archived", key))
		}
		if string(rawValue) == "null" {
			if field.IsRequired {
				return errs.ErrCustomFieldRequired.WithMessage(fmt.Sprintf("field %q is required", key))
			}
			deletes = append(deletes, field.IdCustomField)
			continue
		}
		value, err := model.ParseCustomFieldValue(*field, rawValue)
		if err != nil {
			return err
		}
		value.IdIssue = idIssue
		upserts = append(upserts, value)
	}

	if isCreate {
		if err := s.checkRequired(fields, raw); err != nil {
			return err
		}
	}

	if err := s.valueRepo.DeleteValues(ctx, idIssue, deletes); err != nil {
		return err
	}
	return s.valueRepo.UpsertValues(ctx, upserts)
}

// Only fills keys the caller left out; an explicit JSON null means "empty", which is
// not the same as "unspecified".
func withDefaults(fields []*model.CustomField, raw map[string]json.RawMessage) map[string]json.RawMessage {
	merged := make(map[string]json.RawMessage, len(raw)+len(fields))
	for _, field := range fields {
		if field.IsArchived() || len(field.DefaultValue) == 0 || string(field.DefaultValue) == "null" {
			continue
		}
		merged[field.Key] = field.DefaultValue
	}
	for key, value := range raw {
		merged[key] = value
	}
	return merged
}

func (s *CustomFieldValueService) checkRequired(fields []*model.CustomField, raw map[string]json.RawMessage) error {
	missing := make([]string, 0, len(fields))
	for _, field := range fields {
		if !field.IsRequired || field.IsArchived() {
			continue
		}
		value, sent := raw[field.Key]
		if !sent || string(value) == "null" {
			missing = append(missing, field.Key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return errs.ErrCustomFieldRequired.WithMessage(
		fmt.Sprintf("required custom fields are missing: %s", strings.Join(missing, ", ")))
}
