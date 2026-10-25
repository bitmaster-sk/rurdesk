package service

import (
	"context"
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagService struct {
	pool    *pgxpool.Pool
	tagRepo *repository.TagRepository
}

func NewTagService(pool *pgxpool.Pool, tagRepo *repository.TagRepository) *TagService {
	return &TagService{pool: pool, tagRepo: tagRepo}
}

func (s *TagService) Create(ctx context.Context, req model.CreateTagReq) (*model.Tag, error) {
	name, err := normalizeTagName(req.Name)
	if err != nil {
		return nil, err
	}
	return s.tagRepo.InsertTag(ctx, &model.Tag{
		IdProject: req.IdProject,
		Name:      name,
		Color:     req.Color,
	})
}

func (s *TagService) Update(ctx context.Context, req model.EditTagReq) (*model.Tag, error) {
	name, err := normalizeTagName(req.Name)
	if err != nil {
		return nil, err
	}
	var updated *model.Tag
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		tag, err := s.tagRepo.LoadTag(ctx, req.IdProject, req.IdTag)
		if err != nil {
			return err
		}
		tag.Name = name
		tag.Color = req.Color
		tag.OrderRank = req.OrderRank
		if err := s.tagRepo.UpdateTag(ctx, tag); err != nil {
			return err
		}
		if err := s.tagRepo.MoveTag(ctx, tag); err != nil {
			return err
		}
		updated, err = s.tagRepo.LoadTag(ctx, req.IdProject, req.IdTag)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TagService) Delete(ctx context.Context, idProject, idTag int64) (*model.Tag, error) {
	var deleted *model.Tag
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		tag, err := s.tagRepo.LoadTag(ctx, idProject, idTag)
		if err != nil {
			return err
		}
		deleted = tag
		return s.tagRepo.DeleteTag(ctx, idProject, idTag)
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *TagService) ValidateInProject(ctx context.Context, idProject int64, idsTag []int64) ([]int64, error) {
	unique := slices.Clone(idsTag)
	slices.Sort(unique)
	unique = slices.Compact(unique)
	if len(unique) == 0 {
		return []int64{}, nil
	}
	count, err := s.tagRepo.CountTagsInProject(ctx, idProject, unique)
	if err != nil {
		return nil, err
	}
	if count != len(unique) {
		return nil, errs.ErrTagNotInProject
	}
	return unique, nil
}

func normalizeTagName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errs.ErrValidation.WithMessage("tag name is required")
	}
	return name, nil
}
