package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent/skills"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

type SkillService struct {
	skillRepo        *repository.SkillRepository
	projectSkillRepo *repository.ProjectSkillRepository
}

func NewSkillService(
	skillRepo *repository.SkillRepository,
	projectSkillRepo *repository.ProjectSkillRepository,
) *SkillService {
	return &SkillService{skillRepo: skillRepo, projectSkillRepo: projectSkillRepo}
}

// Every read on this service passes through this before serving.
func (s *SkillService) fillFlags(all ...*model.Skill) {
	for _, skill := range all {
		if skill == nil {
			continue
		}
		skill.IsBuiltin = skill.BuiltinKey != nil
		skill.IsEdited = s.isEdited(skill)
	}
}

func (s *SkillService) isEdited(skill *model.Skill) bool {
	if skill.BuiltinKey == nil {
		return false
	}
	live := skills.Checksum(skill.Name, skill.Description, skill.Content)
	if skill.BuiltinChecksum != nil {
		return live != *skill.BuiltinChecksum
	}
	builtin, ok := skills.BuiltinByKey(*skill.BuiltinKey)
	if !ok {
		return false
	}
	return live != builtin.Checksum()
}

func (s *SkillService) Load(ctx context.Context) ([]*model.Skill, error) {
	all, err := s.skillRepo.Load(ctx)
	if err != nil {
		return nil, err
	}
	s.fillFlags(all...)
	return all, nil
}

func (s *SkillService) LoadById(ctx context.Context, idSkill int64) (*model.Skill, error) {
	skill, err := s.skillRepo.LoadById(ctx, idSkill)
	if err != nil {
		return nil, err
	}
	s.fillFlags(skill)
	return skill, nil
}

func (s *SkillService) LoadByIds(ctx context.Context, idsSkill []int64) ([]*model.Skill, error) {
	all, err := s.skillRepo.LoadByIds(ctx, idsSkill)
	if err != nil {
		return nil, err
	}
	s.fillFlags(all...)
	return all, nil
}

func (s *SkillService) Create(ctx context.Context, dto model.CreateSkillReq) (*model.Skill, error) {
	created, err := s.skillRepo.Insert(ctx, dto.Name, dto.Description, dto.Content)
	if err != nil {
		return nil, err
	}
	s.fillFlags(created)
	return created, nil
}

// A nil field is left as it is, so per-field autosave cannot clobber a field
// someone else is editing.
func (s *SkillService) Update(ctx context.Context, idSkill int64, dto model.UpdateSkillReq) (*model.Skill, error) {
	current, err := s.skillRepo.LoadById(ctx, idSkill)
	if err != nil {
		return nil, err
	}

	name, description, content := current.Name, current.Description, current.Content
	if dto.Name != nil {
		name = *dto.Name
	}
	if dto.Description != nil {
		description = *dto.Description
	}
	if dto.Content != nil {
		content = *dto.Content
	}

	updated, err := s.skillRepo.Update(ctx, idSkill, name, description, content)
	if err != nil {
		return nil, err
	}
	s.fillFlags(updated)
	return updated, nil
}

func (s *SkillService) Delete(ctx context.Context, idSkill int64) error {
	skill, err := s.skillRepo.LoadById(ctx, idSkill)
	if err != nil {
		return err
	}
	if skill.BuiltinKey != nil {
		return errs.ErrSkillBuiltin
	}
	return s.skillRepo.Delete(ctx, idSkill)
}

func (s *SkillService) Restore(ctx context.Context, idSkill int64) (*model.Skill, error) {
	skill, err := s.skillRepo.LoadById(ctx, idSkill)
	if err != nil {
		return nil, err
	}
	if skill.BuiltinKey == nil {
		return nil, errs.ErrSkillNotBuiltin
	}
	builtin, ok := skills.BuiltinByKey(*skill.BuiltinKey)
	if !ok {
		return nil, fmt.Errorf("restoring skill %d: no shipped original for key %q", idSkill, *skill.BuiltinKey)
	}
	restored, err := s.skillRepo.UpdateBuiltin(ctx, idSkill,
		builtin.Name, builtin.Description, builtin.Content, builtin.Checksum())
	if err != nil {
		return nil, err
	}
	s.fillFlags(restored)
	return restored, nil
}

// Untouched builtins follow the shipped text; an edited one is left alone. New
// builtins are seeded onto every project once, so turning one off is permanent.
func (s *SkillService) SyncBuiltins(ctx context.Context) error {
	for _, builtin := range skills.Builtins() {
		idSkill, inserted, err := s.skillRepo.InsertBuiltin(ctx,
			builtin.Key, builtin.Name, builtin.Description, builtin.Content, builtin.Checksum())

		// A custom skill holding the same name must not take the app down.
		if errors.Is(err, errs.ErrSkillNameTaken) {
			log.Warn().Str("builtinKey", builtin.Key).Str("name", builtin.Name).
				Msg("builtin skill name collides with an existing custom skill — skipping")
			continue
		}
		if err != nil {
			return fmt.Errorf("syncing builtin skill %q: %w", builtin.Key, err)
		}

		if !inserted {
			if err := s.refreshBuiltin(ctx, builtin); err != nil {
				return err
			}
			continue
		}
		if err := s.projectSkillRepo.Enable(ctx, idSkill, builtin.DefaultStages); err != nil {
			return fmt.Errorf("enabling builtin skill %q by default: %w", builtin.Key, err)
		}
	}
	return nil
}

func (s *SkillService) refreshBuiltin(ctx context.Context, builtin skills.BuiltinSkill) error {
	skill, err := s.skillRepo.LoadByBuiltinKey(ctx, builtin.Key)
	if errors.Is(err, errs.ErrSkillNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	live := skills.Checksum(skill.Name, skill.Description, skill.Content)
	shipped := builtin.Checksum()

	if skill.BuiltinChecksum == nil {
		if live != shipped {
			return nil
		}
		if err := s.skillRepo.SetChecksum(ctx, skill.IdSkill, shipped); err != nil {
			return fmt.Errorf("adopting checksum of builtin skill %q: %w", builtin.Key, err)
		}
		return nil
	}
	if live != *skill.BuiltinChecksum || live == shipped {
		return nil
	}

	_, err = s.skillRepo.UpdateBuiltin(ctx, skill.IdSkill,
		builtin.Name, builtin.Description, builtin.Content, shipped)
	if errors.Is(err, errs.ErrSkillNameTaken) {
		log.Warn().Str("builtinKey", builtin.Key).Str("name", builtin.Name).
			Msg("updated builtin skill name collides with an existing skill — keeping the previous version")
		return nil
	}
	if err != nil {
		return fmt.Errorf("updating builtin skill %q: %w", builtin.Key, err)
	}
	log.Info().Str("builtinKey", builtin.Key).Msg("builtin skill updated to the shipped version")
	return nil
}
