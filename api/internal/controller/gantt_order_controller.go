package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/lexorank"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GanttOrderController struct {
	issueRepo *repository.IssueRepository
	acl       *service.AclService
	pool      *pgxpool.Pool
}

func NewGanttOrderController(ir *repository.IssueRepository, acl *service.AclService, pool *pgxpool.Pool) *GanttOrderController {
	return &GanttOrderController{issueRepo: ir, acl: acl, pool: pool}
}

// ganttOrderReq carries the moved row and the client's full in-window scheduled
// order. Neighbours are derived server-side from persisted ranks, so a stale
// client can't force a bad midpoint.
type ganttOrderReq struct {
	MovedId int64   `json:"movedId" binding:"required"`
	Order   []int64 `json:"order" binding:"required,min=1"`
}

// Reorder persists a manual order. It seeds evenly-spaced ranks only on a project's
// first drag; afterwards it never reseeds — it anchors on the nearest ranked rows
// around the moved row and midpoint-ranks just the gap. Common case: one write.
// Other ranked rows are never rewritten, preserving their update_at/order.
func (gc *GanttOrderController) Reorder(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	var req ganttOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	err = extctx.RunInTx(ctx, gc.pool, func(ctx context.Context) error {
		if !gc.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errs.ErrForbidden
		}
		// Serializes concurrent reorders of the same project so racing drags can't
		// read stale state and write conflicting ranks. Single-int8 form takes the
		// project id directly; this is the only advisory-lock user today (namespace
		// with a class int if that changes).
		if _, err := extctx.GetDb(ctx, gc.pool).Exec(ctx,
			`SELECT pg_advisory_xact_lock($1)`, idProject); err != nil {
			return err
		}

		ranks, err := gc.issueRepo.LoadScheduledGanttRanks(ctx, idProject)
		if err != nil {
			return err
		}
		byPublic := make(map[int64]repository.GanttRankRow, len(ranks))
		anyRanked := false
		for _, row := range ranks {
			byPublic[row.IdIssuePublic] = row
			if row.GanttRank != nil {
				anyRanked = true
			}
		}

		// Validate: every order id is a scheduled issue of this project, unique,
		// and movedId is among them.
		movedIdx := -1
		seen := make(map[int64]bool, len(req.Order))
		for i, pub := range req.Order {
			if _, ok := byPublic[pub]; !ok || seen[pub] {
				return errs.ErrBadRequest
			}
			seen[pub] = true
			if pub == req.MovedId {
				movedIdx = i
			}
		}
		if movedIdx == -1 {
			return errs.ErrBadRequest
		}

		// First drag ever in this project: seed the whole order once.
		if !anyRanked {
			seeds := lexorank.SeedRanks(len(req.Order))
			for i, pub := range req.Order {
				if err := gc.issueRepo.SetGanttRank(ctx, byPublic[pub].IdIssue, seeds[i]); err != nil {
					return err
				}
			}
			return nil
		}

		// Already ranked: anchor on the nearest ranked rows around movedIdx (walk
		// outward, skipping unranked), then midpoint-rank the unranked gap in place.
		prevRank, nextRank := "", ""
		gapStart, gapEnd := 0, len(req.Order)
		for i := movedIdx - 1; i >= 0; i-- {
			if r := byPublic[req.Order[i]].GanttRank; r != nil {
				prevRank, gapStart = *r, i+1
				break
			}
		}
		for i := movedIdx + 1; i < len(req.Order); i++ {
			if r := byPublic[req.Order[i]].GanttRank; r != nil {
				nextRank, gapEnd = *r, i
				break
			}
		}
		if prevRank != "" && nextRank != "" && prevRank >= nextRank {
			return errs.ErrBadRequest // stale client adjacency (a concurrent reorder moved an anchor)
		}
		lo := prevRank
		for i := gapStart; i < gapEnd; i++ {
			r := lexorank.Between(lo, nextRank) // invariant: lo < r < nextRank, so lo stays < nextRank
			if err := gc.issueRepo.SetGanttRank(ctx, byPublic[req.Order[i]].IdIssue, r); err != nil {
				return err
			}
			lo = r
		}
		return nil
	})

	if err != nil {
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			_ = c.Error(err)
			c.Status(appErr.HttpStatus())
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
