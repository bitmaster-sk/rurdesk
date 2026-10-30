package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

const issueLinkSearchLimit = 8

type IssueLinkService struct {
	acl           *AclService
	issueLinkRepo *repository.IssueLinkRepository
	stateRepo     *repository.StateRepository
	severityRepo  *repository.SeverityRepository
	issueTypeRepo *repository.IssueTypeRepository
	projectRepo   *repository.ProjectRepository
}

func NewIssueLinkService(
	acl *AclService,
	issueLinkRepo *repository.IssueLinkRepository,
	stateRepo *repository.StateRepository,
	severityRepo *repository.SeverityRepository,
	issueTypeRepo *repository.IssueTypeRepository,
	projectRepo *repository.ProjectRepository,
) *IssueLinkService {
	return &IssueLinkService{
		acl:           acl,
		issueLinkRepo: issueLinkRepo,
		stateRepo:     stateRepo,
		severityRepo:  severityRepo,
		issueTypeRepo: issueTypeRepo,
		projectRepo:   projectRepo,
	}
}

func (s *IssueLinkService) Search(ctx context.Context, idUser, idProjectFirst int64, query string) ([]*model.IssueLinkOption, error) {
	idsProject, err := s.acl.LoadVisibleProjectIds(ctx, idUser)
	if err != nil {
		return nil, fmt.Errorf("loading visible projects: %w", err)
	}
	if len(idsProject) == 0 {
		return []*model.IssueLinkOption{}, nil
	}
	query = strings.TrimPrefix(strings.TrimSpace(query), "#")
	return s.issueLinkRepo.Search(ctx, idsProject, idProjectFirst, query, issueLinkSearchLimit)
}

func (s *IssueLinkService) LoadInfo(ctx context.Context, idUser int64, refs []model.IssueLinkRef) (*model.IssueLinkInfoRes, error) {
	res := &model.IssueLinkInfoRes{
		Issues:     []*model.IssueLinkIssue{},
		Projects:   []model.IssueLinkProject{},
		States:     []*model.State{},
		Severities: []*model.Severity{},
		IssueTypes: []*model.IssueType{},
		Users:      []model.IssueLinkUser{},
	}

	projects, err := s.acl.LoadVisibleProjects(ctx, idUser)
	if err != nil {
		return nil, fmt.Errorf("loading visible projects: %w", err)
	}
	namesByProject := make(map[int64]string, len(projects))
	idsVisible := make([]int64, len(projects))
	for i, project := range projects {
		namesByProject[project.IdProject] = project.Name
		idsVisible[i] = project.IdProject
	}

	issues, err := s.issueLinkRepo.LoadByRefs(ctx, idsVisible, uniqueIssueLinkRefs(refs))
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return res, nil
	}
	res.Issues = issues

	var idsProject []int64
	isSeen := make(map[int64]bool)
	for _, issue := range issues {
		if !isSeen[issue.IdProject] {
			isSeen[issue.IdProject] = true
			idsProject = append(idsProject, issue.IdProject)
			res.Projects = append(res.Projects, model.IssueLinkProject{
				IdProject: issue.IdProject,
				Name:      namesByProject[issue.IdProject],
			})
		}
	}

	if res.States, err = s.stateRepo.LoadStates(ctx, idsProject); err != nil {
		return nil, err
	}
	if res.Severities, err = s.severityRepo.LoadSeverities(ctx, idsProject); err != nil {
		return nil, err
	}
	if res.IssueTypes, err = s.issueTypeRepo.LoadIssueTypes(ctx, idsProject); err != nil {
		return nil, err
	}
	members, err := s.projectRepo.LoadProjectsMembers(ctx, idsProject)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		res.Users = append(res.Users, model.IssueLinkUser{
			IdUser:         member.IdUser,
			Name:           member.Name,
			ColorAvatarBg:  member.ColorAvatarBg,
			AvatarInitials: member.AvatarInitials,
		})
	}
	return res, nil
}

func uniqueIssueLinkRefs(refs []model.IssueLinkRef) []model.IssueLinkRef {
	isSeen := make(map[model.IssueLinkRef]bool, len(refs))
	unique := make([]model.IssueLinkRef, 0, len(refs))
	for _, ref := range refs {
		if !isSeen[ref] {
			isSeen[ref] = true
			unique = append(unique, ref)
		}
	}
	return unique
}
