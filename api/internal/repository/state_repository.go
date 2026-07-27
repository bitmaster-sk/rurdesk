package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StateRepository struct {
	pool *pgxpool.Pool
}

func NewStateRepository(pool *pgxpool.Pool) *StateRepository {
	return &StateRepository{pool: pool}
}

func (r *StateRepository) LoadStates(ctx context.Context, idsProject []int64) ([]*model.State, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			sta.id_state,
			pis.id_project,
			sta.name,
			sta.start,
			sta.final,
			sta.protected,
			pis.order_rank
		FROM
			issues.state sta
			INNER JOIN projects.project_issue_state pis ON sta.id_state = pis.id_state
		WHERE
			pis.id_project = ANY($1)
		ORDER BY pis.order_rank
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying states: %w", err)
	}
	states, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.State])
	if err != nil {
		return nil, fmt.Errorf("collecting states: %w", err)
	}
	return states, nil
}

func (r *StateRepository) LoadState(ctx context.Context, idProject int64, idState int64) (*model.State, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			sta.id_state,
			pis.id_project,
			sta.name,
			sta.start,
			sta.final,
			sta.protected,
			pis.order_rank
		FROM
			issues.state sta
			INNER JOIN projects.project_issue_state pis ON sta.id_state = pis.id_state
		WHERE
			pis.id_project = $1 AND
			sta.id_state = $2
	`, idProject, idState)
	if err != nil {
		return nil, fmt.Errorf("querying state: %w", err)
	}
	state, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.State])
	if err != nil {
		return nil, fmt.Errorf("collecting state: %w", err)
	}
	return state, nil
}

func (r *StateRepository) InsertDefaultStates(ctx context.Context, idProject int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO projects.project_issue_state (id_project, id_state, order_rank)
		SELECT $1, id_state, ROW_NUMBER() OVER (ORDER BY id_state) FROM issues.state WHERE protected = true ORDER BY id_state
	`, idProject)
	if err != nil {
		return fmt.Errorf("inserting default states: %w", err)
	}
	return nil
}

func (r *StateRepository) InsertState(ctx context.Context, state *model.State) (*model.State, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.state(name, start, final, protected)
		VALUES($1, $2, $3, false) RETURNING id_state
	`, state.Name, state.Start, state.Final).Scan(&state.IdState)
	if err != nil {
		return nil, fmt.Errorf("inserting state: %w", err)
	}
	state.Protected = false
	return state, nil
}

func (r *StateRepository) UpdateState(ctx context.Context, state *model.State) (*model.State, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.state SET name = $1, start = $2, final = $3 WHERE id_state = $4
	`, state.Name, state.Start, state.Final, state.IdState)
	if err != nil {
		return nil, fmt.Errorf("updating state: %w", err)
	}
	return state, nil
}

func (r *StateRepository) DeleteState(ctx context.Context, idState int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.state WHERE id_state = $1`, idState)
	if err != nil {
		return fmt.Errorf("deleting state: %w", err)
	}
	return nil
}

// ReassignIssuesState repoints every issue in a project from one state to
// another (or to NULL when newIdState is nil) — e.g. when a protected mapping
// is removed, so issues aren't orphaned on a shared row. Scoped to id_project
// so issues in other projects sharing the (protected) row are untouched.
func (r *StateRepository) ReassignIssuesState(ctx context.Context, idProject, oldIdState int64, newIdState *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue SET id_state = $3 WHERE id_project = $1 AND id_state = $2
	`, idProject, oldIdState, newIdState)
	if err != nil {
		return fmt.Errorf("reassigning issues state: %w", err)
	}
	return nil
}

func (r *StateRepository) InsertProjectState(ctx context.Context, state *model.State) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO projects.project_issue_state (id_project, id_state, order_rank)
		SELECT $1, $2, COALESCE(MAX(order_rank), 0) + 1 FROM projects.project_issue_state WHERE id_project = $1
	`, state.IdProject, state.IdState)
	if err != nil {
		return fmt.Errorf("inserting project state: %w", err)
	}
	return nil
}

func (r *StateRepository) DeleteProjectState(ctx context.Context, state *model.State) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM projects.project_issue_state WHERE id_project = $1 AND id_state = $2
	`, state.IdProject, state.IdState)
	if err != nil {
		return fmt.Errorf("deleting project state: %w", err)
	}
	return r.reorder(ctx, state, 0)
}

func (r *StateRepository) UpdateProjectState(ctx context.Context, state *model.State) error {
	db := extctx.GetDb(ctx, r.pool)
	var oldOrderRank int64
	err := db.QueryRow(ctx, `
		UPDATE projects.project_issue_state new SET
			order_rank = $3
		FROM projects.project_issue_state old
		WHERE
			old.id_project = new.id_project AND
			old.id_project = $2 AND
			old.id_state = new.id_state AND
			old.id_state = $1
		RETURNING old.order_rank
	`, state.IdState, state.IdProject, state.OrderRank).Scan(&oldOrderRank)
	if err != nil {
		return fmt.Errorf("updating project state: %w", err)
	}
	return r.reorder(ctx, state, oldOrderRank)
}

// reorder renumbers order_rank sequentially after an insert/delete/move.
func (r *StateRepository) reorder(ctx context.Context, state *model.State, oldOrderRank int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE projects.project_issue_state des SET
			order_rank = src.order_rank
		FROM (
			SELECT
				src_pis.id_project,
				src_pis.id_state,
				ROW_NUMBER() OVER (
					ORDER BY
						order_rank,
						CASE
							WHEN src_pis.id_state = $1 AND src_pis.order_rank > $3 THEN 1
							WHEN src_pis.id_state = $1 AND src_pis.order_rank < $3 THEN -1
							ELSE 0
						END
				) AS order_rank
			FROM projects.project_issue_state src_pis
			WHERE src_pis.id_project = $2
		) src
		WHERE des.id_project = src.id_project AND des.id_state = src.id_state AND des.id_project = $2
	`, state.IdState, state.IdProject, oldOrderRank)
	if err != nil {
		return fmt.Errorf("reordering project states: %w", err)
	}
	return nil
}

// FinalStateIds returns the ids of the project's states flagged final. States
// link to projects via projects.project_issue_state (issues.state has no
// id_project column), so join through it.
func (r *StateRepository) FinalStateIds(ctx context.Context, idProject int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT sta.id_state
		FROM issues.state sta
		INNER JOIN projects.project_issue_state pis ON sta.id_state = pis.id_state
		WHERE pis.id_project = $1 AND sta.final = TRUE
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying final state ids: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting final state ids: %w", err)
	}
	return ids, nil
}
