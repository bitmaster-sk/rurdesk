package service

import (
	"context"
	"errors"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

// SplitService handles AI-driven issue splitting.
type SplitService struct {
	pool      *pgxpool.Pool
	aiClient  ai.Provider
	issueRepo *repository.IssueRepository
	relRepo   *repository.IssueRelationRepository
	stateRepo *repository.StateRepository
}

func NewSplitService(
	pool *pgxpool.Pool,
	aiClient ai.Provider,
	issueRepo *repository.IssueRepository,
	relRepo *repository.IssueRelationRepository,
	stateRepo *repository.StateRepository,
) *SplitService {
	return &SplitService{
		pool:      pool,
		aiClient:  aiClient,
		issueRepo: issueRepo,
		relRepo:   relRepo,
		stateRepo: stateRepo,
	}
}

// Preview calls the AI provider and returns proposed child issues without persisting.
func (s *SplitService) Preview(ctx context.Context, idIssue int64, hint string) ([]model.ProposedIssue, error) {
	aiModel := viper.GetString("AI_MODEL")
	if aiModel == "" {
		return nil, errs.ErrAiNotConfigured
	}

	issue, err := s.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	messages := ai.BuildSplitPrompt(issue.Title, issue.Description, "", hint, issue.Estimated)

	res, err := s.aiClient.Complete(ctx, ai.CompletionReq{
		Model:     aiModel,
		Messages:  messages,
		Tools:     ai.SplitTools(),
		MaxTokens: 32768,
	})
	if err != nil {
		return nil, errs.ErrAiUnavailable
	}

	proposed, err := ai.ParseSplitResponse(res)
	if err != nil {
		return nil, errs.ErrAiInvalidResponse
	}

	return proposed, nil
}

// Accept creates all child issues and hierarchy relations in a single transaction.
func (s *SplitService) Accept(ctx context.Context, idProject, idIssue int64, children []model.ProposedIssue, createdBy int64) ([]*model.Issue, error) {
	if len(children) == 0 {
		return nil, errs.ErrBadRequest
	}

	parent, err := s.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	if parent.IdProject != idProject {
		return nil, errs.ErrNotFound
	}

	states, err := s.stateRepo.LoadStates(ctx, []int64{idProject})
	if err != nil {
		return nil, err
	}

	var idDefaultState *int64
	if len(states) > 0 {
		idDefaultState = &states[0].IdState
	}

	var created []*model.Issue

	txErr := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		for _, child := range children {
			estimated := int64(0)
			if child.EstimatedMinutes != nil {
				estimated = *child.EstimatedMinutes
			}

			newIssue := &model.Issue{
				IdProject:   idProject,
				IdState:     idDefaultState,
				Title:       child.Title,
				Description: child.Description,
				Estimated:   estimated,
				CreateBy:    createdBy,
				UpdateBy:    createdBy,
			}

			inserted, err := s.issueRepo.InsertIssue(ctx, newIssue)
			if err != nil {
				return err
			}

			_, err = s.relRepo.InsertRelation(ctx, &model.IssueRelation{
				IdProject:    idProject,
				IdIssueFrom:  parent.IdIssue,
				IdIssueTo:    inserted.IdIssue,
				RelationType: model.RelationTypeHierarchy,
				CreatedBy:    createdBy,
			})
			if err != nil {
				return err
			}

			created = append(created, inserted)
		}
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return created, nil
}
