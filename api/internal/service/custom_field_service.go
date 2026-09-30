package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OptionIntent struct {
	MigrateTo    *int64
	DeleteValues bool
}

type CustomFieldService struct {
	pool            *pgxpool.Pool
	customFieldRepo *repository.CustomFieldRepository
	valueRepo       *repository.CustomFieldValueRepository
}

func NewCustomFieldService(
	pool *pgxpool.Pool,
	customFieldRepo *repository.CustomFieldRepository,
	valueRepo *repository.CustomFieldValueRepository,
) *CustomFieldService {
	return &CustomFieldService{pool: pool, customFieldRepo: customFieldRepo, valueRepo: valueRepo}
}

func (s *CustomFieldService) Create(ctx context.Context, req model.CreateCustomFieldReq) (*model.CustomField, error) {
	if !constants.IsValidCustomFieldType(req.FieldType) {
		return nil, errs.ErrValidation.WithMessage("unsupported field type")
	}
	if req.FieldType != constants.CustomFieldTypeSelect && len(req.Options) > 0 {
		return nil, errs.ErrValidation.WithMessage("only select fields can have options")
	}
	if req.FieldType == constants.CustomFieldTypeSelect && len(req.Options) == 0 {
		return nil, errs.ErrValidation.WithMessage("a select field needs at least one option")
	}
	if req.FieldType == constants.CustomFieldTypeSelect && hasDefaultValue(req.DefaultValue) {
		return nil, errs.ErrValidation.WithMessage(
			"a select field can only get a default value once its options exist")
	}
	defaultValue, err := validatedDefaultValue(
		model.CustomField{Key: req.Key, FieldType: req.FieldType}, req.DefaultValue)
	if err != nil {
		return nil, err
	}

	var created *model.CustomField
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		exists, err := s.customFieldRepo.ExistsKey(ctx, req.IdProject, req.Key)
		if err != nil {
			return err
		}
		if exists {
			return errs.ErrCustomFieldKeyExists
		}
		field, err := s.customFieldRepo.InsertCustomField(ctx, &model.CustomField{
			IdProject:     req.IdProject,
			Key:           req.Key,
			Name:          req.Name,
			FieldType:     req.FieldType,
			IsRequired:    req.IsRequired,
			RequiredSince: requiredSince(req.IsRequired, nil),
			DefaultValue:  defaultValue,
		})
		if err != nil {
			return err
		}
		field.Options = []model.CustomFieldOption{}
		for index, option := range req.Options {
			inserted, err := s.customFieldRepo.InsertOption(ctx, field.IdCustomField, option.Label, index+1)
			if err != nil {
				return err
			}
			field.Options = append(field.Options, *inserted)
		}
		created = field
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *CustomFieldService) Update(ctx context.Context, req model.EditCustomFieldReq, intent OptionIntent) (*model.CustomField, error) {
	var updated *model.CustomField
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		field, err := s.customFieldRepo.LoadCustomField(ctx, req.IdProject, req.IdCustomField)
		if err != nil {
			return err
		}
		if req.Key != "" && req.Key != field.Key {
			return errs.ErrCustomFieldImmutable.WithMessage("custom field key cannot be changed")
		}
		if req.FieldType != "" && req.FieldType != field.FieldType {
			return errs.ErrCustomFieldImmutable.WithMessage("custom field type cannot be changed")
		}

		field.Name = req.Name
		if req.IsRequired.IsDefined {
			field.IsRequired = req.IsRequired.OrElse(false)
			field.RequiredSince = requiredSince(field.IsRequired, field.RequiredSince)
		}

		if req.IsArchived.IsDefined {
			if req.IsArchived.OrElse(false) {
				if field.ArchivedAt == nil {
					now := time.Now().UTC()
					field.ArchivedAt = &now
				}
			} else {
				field.ArchivedAt = nil
			}
		}
		if err := s.customFieldRepo.UpdateCustomField(ctx, field); err != nil {
			return err
		}
		if req.OrderRank > 0 && req.OrderRank != field.OrderRank {
			field.OrderRank = req.OrderRank
			if err := s.customFieldRepo.MoveCustomField(ctx, field); err != nil {
				return err
			}
		}

		// A nil Options means "not sent" and must leave options alone; collapsing
		// it with an empty slice would let a plain rename wipe them.
		if req.Options != nil {
			if field.FieldType != constants.CustomFieldTypeSelect {
				return errs.ErrValidation.WithMessage("only select fields can have options")
			}
			if len(req.Options) == 0 {
				return errs.ErrValidation.WithMessage("a select field needs at least one option")
			}
			if err := s.applyOptions(ctx, field, req.Options, intent); err != nil {
				return err
			}
		}

		updated, err = s.customFieldRepo.LoadCustomField(ctx, req.IdProject, req.IdCustomField)
		if err != nil {
			return err
		}
		// Must stay after the options: a select default points at an option id that
		// only exists once they are written.
		if req.DefaultValue != nil {
			updated.DefaultValue, err = validatedDefaultValue(*updated, req.DefaultValue)
			if err != nil {
				return err
			}
			if err := s.customFieldRepo.UpdateCustomField(ctx, updated); err != nil {
				return err
			}
		}
		return s.backfill(ctx, updated, req.Backfill)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *CustomFieldService) DeleteWithConfirm(ctx context.Context, idProject, idCustomField int64, deleteValues bool) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		field, err := s.customFieldRepo.LoadCustomField(ctx, idProject, idCustomField)
		if err != nil {
			return err
		}
		usage, err := s.customFieldRepo.LoadCustomFieldUsage(ctx, idProject, idCustomField)
		if err != nil {
			return err
		}
		if usage.Issues > 0 && !deleteValues {
			return errs.ErrCustomFieldInUse.WithMessage(
				fmt.Sprintf("field %q has values in %d issues", field.Key, usage.Issues))
		}
		return s.customFieldRepo.DeleteCustomField(ctx, idProject, idCustomField)
	})
}

func (s *CustomFieldService) applyOptions(
	ctx context.Context,
	field *model.CustomField,
	requested []model.CustomFieldOptionReq,
	intent OptionIntent,
) error {
	usage, err := s.customFieldRepo.LoadCustomFieldUsage(ctx, field.IdProject, field.IdCustomField)
	if err != nil {
		return err
	}

	kept := make(map[int64]bool, len(requested))
	for _, option := range requested {
		if option.IdOption == 0 {
			continue
		}
		// Without this, an id the field does not own gets written through to another
		// project's option.
		if !field.HasOption(option.IdOption) {
			return errs.ErrValidation.WithMessage(
				fmt.Sprintf("option %d does not belong to field %q", option.IdOption, field.Key))
		}
		kept[option.IdOption] = true
	}

	for _, existing := range field.Options {
		if kept[existing.IdOption] {
			continue
		}
		if usage.OptionUsage[existing.IdOption] > 0 {
			if err := s.resolveRemovedOption(ctx, field, existing.IdOption, kept, intent); err != nil {
				return err
			}
		}
		if err := s.customFieldRepo.DeleteOption(ctx, field.IdCustomField, existing.IdOption); err != nil {
			return err
		}
		// No foreign key links the default to the option row, so a dangling default
		// would survive and reject every new issue.
		if s.isDefaultOption(field, existing.IdOption) {
			field.DefaultValue = nil
			if err := s.customFieldRepo.UpdateCustomField(ctx, field); err != nil {
				return err
			}
		}
	}

	for index, option := range requested {
		if option.IdOption == 0 {
			if _, err := s.customFieldRepo.InsertOption(ctx, field.IdCustomField, option.Label, index+1); err != nil {
				return err
			}
			continue
		}
		if err := s.customFieldRepo.UpdateOption(ctx, field.IdCustomField, option.IdOption, option.Label, index+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *CustomFieldService) isDefaultOption(field *model.CustomField, idOption int64) bool {
	if field.FieldType != constants.CustomFieldTypeSelect || !hasDefaultValue(field.DefaultValue) {
		return false
	}
	var idDefaultOption int64
	if err := json.Unmarshal(field.DefaultValue, &idDefaultOption); err != nil {
		return false
	}
	return idDefaultOption == idOption
}

func (s *CustomFieldService) resolveRemovedOption(
	ctx context.Context,
	field *model.CustomField,
	idOption int64,
	kept map[int64]bool,
	intent OptionIntent,
) error {
	switch {
	case intent.MigrateTo != nil:
		if *intent.MigrateTo == idOption || !kept[*intent.MigrateTo] {
			return errs.ErrInvalidCustomFieldOptionMigrationTarget
		}
		return s.customFieldRepo.ReassignOptionValues(ctx, field.IdCustomField, idOption, intent.MigrateTo)
	case intent.DeleteValues:
		if field.IsRequired {
			return errs.ErrCustomFieldRequired.WithMessage(
				fmt.Sprintf("values of required field %q cannot be dropped, migrate them instead", field.Key))
		}
		return s.valueRepo.DeleteValuesOfOption(ctx, field.IdCustomField, idOption)
	default:
		return errs.ErrCustomFieldOptionInUse.WithMessage(
			fmt.Sprintf("option %d of field %q still has values", idOption, field.Key))
	}
}

func (s *CustomFieldService) backfill(ctx context.Context, field *model.CustomField, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if field.IsArchived() {
		return errs.ErrCustomFieldArchived.WithMessage(fmt.Sprintf("field %q is archived", field.Key))
	}
	value, err := model.ParseCustomFieldValue(*field, raw)
	if err != nil {
		return err
	}
	return s.valueRepo.BackfillValues(ctx, field.IdProject, value)
}

func hasDefaultValue(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

func validatedDefaultValue(field model.CustomField, raw json.RawMessage) (json.RawMessage, error) {
	if !hasDefaultValue(raw) {
		return nil, nil
	}
	if _, err := model.ParseCustomFieldValue(field, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func requiredSince(isRequired bool, current *time.Time) *time.Time {
	if !isRequired {
		return nil
	}
	if current != nil {
		return current
	}
	now := time.Now().UTC()
	return &now
}
