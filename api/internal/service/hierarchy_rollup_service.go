package service

import (
	"context"
	"slices"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

const maxHierarchyDepth = 100

type HierarchyRollupService struct {
	repo       *repository.IssueHierarchyRepository
	historySvc *IssueHistoryService
}

func NewHierarchyRollupService(repo *repository.IssueHierarchyRepository, historySvc *IssueHistoryService) *HierarchyRollupService {
	return &HierarchyRollupService{repo: repo, historySvc: historySvc}
}

func (s *HierarchyRollupService) RecomputeAncestors(ctx context.Context, idsIssue []int64) ([]int64, error) {
	parents, err := s.repo.LoadParentIds(ctx, idsIssue)
	if err != nil {
		return nil, err
	}
	return s.RecomputeFrom(ctx, parents)
}

func (s *HierarchyRollupService) RecomputeSelfAndAncestors(ctx context.Context, idIssue int64) ([]int64, error) {
	own, err := s.RecomputeFrom(ctx, []int64{idIssue})
	if err != nil {
		return nil, err
	}
	above, err := s.RecomputeAncestors(ctx, []int64{idIssue})
	if err != nil {
		return nil, err
	}
	for _, id := range above {
		if !slices.Contains(own, id) {
			own = append(own, id)
		}
	}
	return own, nil
}

// A node reached by two chains of different length is recomputed once per chain;
// only the last pass, after its deepest child settled, is final.
func (s *HierarchyRollupService) RecomputeFrom(ctx context.Context, idsIssue []int64) ([]int64, error) {
	seen := make(map[int64]bool)
	var changedAll []int64
	level := idsIssue
	for depth := 0; len(level) > 0 && depth < maxHierarchyDepth; depth++ {
		recomputed, err := s.repo.RecomputeNodes(ctx, level)
		if err != nil {
			return nil, err
		}
		changed := make([]int64, 0, len(recomputed))
		for _, issue := range recomputed {
			if err := s.historySvc.RecordSystem(ctx, issue.IdIssue, issue.HistoryChanges()); err != nil {
				return nil, err
			}
			changed = append(changed, issue.IdIssue)
		}
		for _, id := range changed {
			if !seen[id] {
				seen[id] = true
				changedAll = append(changedAll, id)
			}
		}
		level, err = s.repo.LoadParentIds(ctx, changed)
		if err != nil {
			return nil, err
		}
	}
	return changedAll, nil
}

func (s *HierarchyRollupService) RejectScheduleCycle(ctx context.Context, idProject int64, idsRoot []int64) error {
	cycle, err := s.repo.HasScheduleCycleThroughHierarchy(ctx, idProject, idsRoot)
	if err != nil {
		return err
	}
	if cycle {
		return errs.ErrCycle
	}
	return nil
}

func (s *HierarchyRollupService) RejectScheduleOnOwnBranch(ctx context.Context, idFrom, idTo int64) error {
	same, err := s.repo.IsOnSameBranch(ctx, idFrom, idTo)
	if err != nil {
		return err
	}
	if same {
		return errs.ErrCycle
	}
	return nil
}

func (s *HierarchyRollupService) LockIssuesInOrder(ctx context.Context, idProject int64, idsIssuePublic []int64) error {
	return s.repo.LockIssuesInOrder(ctx, idProject, idsIssuePublic)
}

func (s *HierarchyRollupService) ParentIds(ctx context.Context, idsIssue []int64) ([]int64, error) {
	return s.repo.LoadParentIds(ctx, idsIssue)
}
