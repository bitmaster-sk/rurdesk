// Package adminops holds maintenance operations invoked by the `admin` CLI
// (cmd/admin) — one-off tasks an operator runs against a live deployment, kept
// out of the request path.
package adminops

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RotateGitTokens re-encrypts every stored git integration token from oldKey to
// newKey inside a single transaction and returns how many were rotated. Both
// keys must be 32 bytes (AES-256). If any token fails to decrypt with oldKey the
// whole batch rolls back, so a wrong old key leaves the data untouched. The read
// takes FOR UPDATE inside that transaction: a token written between the read and
// the commit would stay under oldKey and become unreadable once oldKey is gone.
func RotateGitTokens(ctx context.Context, pool *pgxpool.Pool, oldKey, newKey []byte) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	rows, err := tx.Query(ctx, `
		SELECT id_git_integration, access_token_enc, token_nonce
		FROM projects.git_integration
		ORDER BY id_git_integration
		FOR UPDATE
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id         int64
		ciphertext []byte
		nonce      []byte
	}
	var toRotate []row
	for rows.Next() {
		var r row
		if scanErr := rows.Scan(&r.id, &r.ciphertext, &r.nonce); scanErr != nil {
			return 0, scanErr
		}
		toRotate = append(toRotate, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close() // the batch below reuses this transaction's connection

	batch := &pgx.Batch{}
	for _, r := range toRotate {
		plaintext, decryptErr := githost.Decrypt(oldKey, r.nonce, r.ciphertext)
		if decryptErr != nil {
			return 0, decryptErr
		}
		newCiphertext, newNonce, encryptErr := githost.Encrypt(newKey, plaintext)
		if encryptErr != nil {
			return 0, encryptErr
		}
		batch.Queue(`
			UPDATE projects.git_integration
			SET access_token_enc = $1, token_nonce = $2, updated_at = NOW()
			WHERE id_git_integration = $3
		`, newCiphertext, newNonce, r.id)
	}

	results := tx.SendBatch(ctx, batch)
	for range toRotate {
		if _, execErr := results.Exec(); execErr != nil {
			_ = results.Close()
			return 0, execErr
		}
	}
	if err := results.Close(); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return len(toRotate), nil
}
