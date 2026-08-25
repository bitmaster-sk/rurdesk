package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrGitIntegrationDuplicate = errors.New("git integration duplicate")

type GitIntegrationRepository struct {
	pool *pgxpool.Pool
}

func NewGitIntegrationRepository(pool *pgxpool.Pool) *GitIntegrationRepository {
	return &GitIntegrationRepository{pool: pool}
}

func (r *GitIntegrationRepository) ListByProject(ctx context.Context, idProject int64) ([]*model.GitIntegration, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_git_integration, id_project, name, host_type, base_url, repo_path,
		       access_token_enc, token_nonce, created_at, updated_at
		FROM projects.git_integration
		WHERE id_project = $1
		ORDER BY created_at ASC
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("listing git integrations: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.GitIntegration])
}

func (r *GitIntegrationRepository) LoadByID(ctx context.Context, idGitIntegration, idProject int64) (*model.GitIntegration, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_git_integration, id_project, name, host_type, base_url, repo_path,
		       access_token_enc, token_nonce, created_at, updated_at
		FROM projects.git_integration
		WHERE id_git_integration = $1 AND id_project = $2
	`, idGitIntegration, idProject)
	if err != nil {
		return nil, fmt.Errorf("loading git integration %d: %w", idGitIntegration, err)
	}
	integration, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.GitIntegration])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return integration, err
}

func (r *GitIntegrationRepository) Create(ctx context.Context, idProject int64, name, hostType, baseUrl, repoPath string, tokenEnc, nonce []byte) (*model.GitIntegration, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		INSERT INTO projects.git_integration
		    (id_project, name, host_type, base_url, repo_path, access_token_enc, token_nonce)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id_git_integration, id_project, name, host_type, base_url, repo_path,
		          access_token_enc, token_nonce, created_at, updated_at
	`, idProject, name, hostType, baseUrl, repoPath, tokenEnc, nonce)
	if err != nil {
		return nil, fmt.Errorf("creating git integration: %w", err)
	}
	integration, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.GitIntegration])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrGitIntegrationDuplicate
		}
		return nil, fmt.Errorf("scanning created integration: %w", err)
	}
	return integration, nil
}

func (r *GitIntegrationRepository) Update(ctx context.Context, idGitIntegration, idProject int64, name, hostType, baseUrl, repoPath string, tokenEnc, nonce []byte) (*model.GitIntegration, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		UPDATE projects.git_integration SET
		    name             = $1,
		    host_type        = $2,
		    base_url         = $3,
		    repo_path        = $4,
		    access_token_enc = COALESCE($5, access_token_enc),
		    token_nonce      = COALESCE($6, token_nonce),
		    updated_at       = NOW()
		WHERE id_git_integration = $7 AND id_project = $8
		RETURNING id_git_integration, id_project, name, host_type, base_url, repo_path,
		          access_token_enc, token_nonce, created_at, updated_at
	`, name, hostType, baseUrl, repoPath, tokenEnc, nonce, idGitIntegration, idProject)
	if err != nil {
		return nil, fmt.Errorf("updating git integration %d: %w", idGitIntegration, err)
	}
	integration, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.GitIntegration])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return integration, err
}

// Delete removes the integration. It first NULLs id_git_integration and
// mr_id on linked issues — FK ON DELETE SET NULL only nulls one column,
// which would violate the both-or-neither CHECK — then deletes the
// integration row. Returns the count of unlinked issues.
func (r *GitIntegrationRepository) Delete(ctx context.Context, idGitIntegration, idProject int64) (int64, error) {
	var unlinkedCount int64
	err := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		db := extctx.GetDb(ctx, r.pool)

		tag, err := db.Exec(ctx, `
			UPDATE issues.issue
			SET id_git_integration = NULL, mr_id = NULL, mr_state = NULL
			WHERE id_git_integration = $1
		`, idGitIntegration)
		if err != nil {
			return fmt.Errorf("unlinking issues for integration %d: %w", idGitIntegration, err)
		}
		unlinkedCount = tag.RowsAffected()

		tag, err = db.Exec(ctx, `
			DELETE FROM projects.git_integration
			WHERE id_git_integration = $1 AND id_project = $2
		`, idGitIntegration, idProject)
		if err != nil {
			return fmt.Errorf("deleting integration %d: %w", idGitIntegration, err)
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return unlinkedCount, nil
}
