package service

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type ProjectMemberService struct {
	projectRepo *repository.ProjectRepository
}

func NewProjectMemberService(projectRepo *repository.ProjectRepository) *ProjectMemberService {
	return &ProjectMemberService{projectRepo: projectRepo}
}

func (s *ProjectMemberService) LoadMemberIds(ctx context.Context, idProject int64) ([]int64, error) {
	members, err := s.projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err != nil {
		return nil, err
	}
	idsUser := make([]int64, len(members))
	for i, member := range members {
		idsUser[i] = member.IdUser
	}
	return idsUser, nil
}
