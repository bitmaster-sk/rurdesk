package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent/skills"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type ProjectSkillService struct {
	projectSkillRepo *repository.ProjectSkillRepository
	skillRepo        *repository.SkillRepository
}

func NewProjectSkillService(
	projectSkillRepo *repository.ProjectSkillRepository,
	skillRepo *repository.SkillRepository,
) *ProjectSkillService {
	return &ProjectSkillService{projectSkillRepo: projectSkillRepo, skillRepo: skillRepo}
}

func (s *ProjectSkillService) LoadDefaultIdsSkillByStage(ctx context.Context, idProject int64) (map[string][]int64, error) {
	all, err := s.projectSkillRepo.Load(ctx, idProject)
	if err != nil {
		return nil, err
	}
	byStage := make(map[string][]int64, len(all))
	for _, entry := range all {
		byStage[entry.Stage] = append(byStage[entry.Stage], entry.IdSkill)
	}
	return byStage, nil
}

func (s *ProjectSkillService) Replace(ctx context.Context, idProject int64, entries []model.UpdateProjectSkillReq) ([]*model.ProjectSkill, error) {
	if err := s.assertSkillsExist(ctx, entries); err != nil {
		return nil, err
	}
	return s.projectSkillRepo.Replace(ctx, idProject, entries)
}

func (s *ProjectSkillService) assertSkillsExist(ctx context.Context, entries []model.UpdateProjectSkillReq) error {
	if len(entries) == 0 {
		return nil
	}
	wanted := make(map[int64]bool, len(entries))
	idsSkill := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if wanted[entry.IdSkill] {
			continue
		}
		wanted[entry.IdSkill] = true
		idsSkill = append(idsSkill, entry.IdSkill)
	}

	found, err := s.skillRepo.LoadByIds(ctx, idsSkill)
	if err != nil {
		return err
	}
	if len(found) == len(idsSkill) {
		return nil
	}
	for _, skill := range found {
		delete(wanted, skill.IdSkill)
	}
	for idSkill := range wanted {
		return errs.ErrUnknownSkill.WithMessage(fmt.Sprintf("skill %d does not exist", idSkill))
	}
	return nil
}

// Existing rows are never touched, so a skill the project turned off stays off.
func (s *ProjectSkillService) SeedDefaults(ctx context.Context, idProject int64) error {
	for _, builtin := range skills.Builtins() {
		for _, stage := range builtin.DefaultStages {
			if err := s.projectSkillRepo.EnableForProject(ctx, idProject, builtin.Key, stage); err != nil {
				return err
			}
		}
	}
	return nil
}
