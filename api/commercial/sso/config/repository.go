package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	connectionColumns = `protocol, issuer, client_id, client_secret_enc, client_secret_nonce,
	scopes_groups, email_claim, groups_claim, idp_metadata_url, idp_metadata_xml, idp_metadata_fetched_at,
	name_attribute, subject_attribute, tested_hash, update_at`
	copiedColumns = `protocol, issuer, client_id, client_secret_enc, client_secret_nonce,
	scopes_groups, email_claim, groups_claim, idp_metadata_url, idp_metadata_xml, idp_metadata_fetched_at,
	name_attribute, subject_attribute`
)

type repository struct {
	pool *pgxpool.Pool
}

func (r *repository) loadConnection(ctx context.Context, kind string) (*storedConnection, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+connectionColumns+`
		FROM users.sso_config
		WHERE kind = $1
	`, kind)
	if err != nil {
		return nil, fmt.Errorf("querying sso config: %w", err)
	}
	stored, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[storedConnection])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("collecting sso config: %w", err)
	}
	return stored, nil
}

func (r *repository) saveDraft(ctx context.Context, idUser int64, stored *storedConnection) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO users.sso_config (kind, `+copiedColumns+`, update_at, update_by)
		VALUES ('draft', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now(), $14)
		ON CONFLICT (kind) DO UPDATE SET
			protocol = EXCLUDED.protocol,
			issuer = EXCLUDED.issuer,
			client_id = EXCLUDED.client_id,
			client_secret_enc = EXCLUDED.client_secret_enc,
			client_secret_nonce = EXCLUDED.client_secret_nonce,
			scopes_groups = EXCLUDED.scopes_groups,
			email_claim = EXCLUDED.email_claim,
			groups_claim = EXCLUDED.groups_claim,
			idp_metadata_url = EXCLUDED.idp_metadata_url,
			idp_metadata_xml = EXCLUDED.idp_metadata_xml,
			idp_metadata_fetched_at = EXCLUDED.idp_metadata_fetched_at,
			name_attribute = EXCLUDED.name_attribute,
			subject_attribute = EXCLUDED.subject_attribute,
			update_at = now(),
			update_by = EXCLUDED.update_by
	`, stored.Protocol, stored.Issuer, stored.ClientId, stored.ClientSecretEnc, stored.ClientSecretNonce,
		stored.HasGroupsScope, stored.EmailClaim, stored.GroupsClaim, stored.MetadataUrl, stored.MetadataXml,
		stored.MetadataFetchedAt, stored.NameAttribute, stored.SubjectAttribute, idUser)
	if err != nil {
		return fmt.Errorf("saving sso draft: %w", err)
	}
	return nil
}

func (r *repository) lockDraft(ctx context.Context) error {
	var one int
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `SELECT 1 FROM users.sso_config WHERE kind = 'draft' FOR UPDATE`).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotConfigured
	}
	if err != nil {
		return fmt.Errorf("locking sso draft: %w", err)
	}
	return nil
}

func (r *repository) setTestedHash(ctx context.Context, hash string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.sso_config SET tested_hash = $1 WHERE kind = 'draft'`, hash)
	if err != nil {
		return fmt.Errorf("saving sso tested hash: %w", err)
	}
	return nil
}

func (r *repository) copyDraftToActive(ctx context.Context, idUser int64) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO users.sso_config (kind, `+copiedColumns+`, tested_hash, update_at, update_by)
		SELECT 'active', `+copiedColumns+`, tested_hash, now(), $1
		FROM users.sso_config
		WHERE kind = 'draft'
		ON CONFLICT (kind) DO UPDATE SET
			protocol = EXCLUDED.protocol,
			issuer = EXCLUDED.issuer,
			client_id = EXCLUDED.client_id,
			client_secret_enc = EXCLUDED.client_secret_enc,
			client_secret_nonce = EXCLUDED.client_secret_nonce,
			scopes_groups = EXCLUDED.scopes_groups,
			email_claim = EXCLUDED.email_claim,
			groups_claim = EXCLUDED.groups_claim,
			idp_metadata_url = EXCLUDED.idp_metadata_url,
			idp_metadata_xml = EXCLUDED.idp_metadata_xml,
			idp_metadata_fetched_at = EXCLUDED.idp_metadata_fetched_at,
			name_attribute = EXCLUDED.name_attribute,
			subject_attribute = EXCLUDED.subject_attribute,
			tested_hash = EXCLUDED.tested_hash,
			update_at = now(),
			update_by = EXCLUDED.update_by
	`, idUser)
	if err != nil {
		return fmt.Errorf("activating sso draft: %w", err)
	}
	return nil
}

func (r *repository) loadSettings(ctx context.Context) (Settings, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT s.is_enabled, s.allowed_domains, s.is_auto_create, s.admin_group, s.is_enforced,
			coalesce(c.protocol, '') AS active_protocol
		FROM users.sso_settings s
		LEFT JOIN users.sso_config c ON c.kind = 'active'
	`)
	if err != nil {
		return Settings{}, fmt.Errorf("querying sso settings: %w", err)
	}
	settings, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Settings])
	if err != nil {
		return Settings{}, fmt.Errorf("collecting sso settings: %w", err)
	}
	return settings, nil
}

func (r *repository) setEnabled(ctx context.Context, isEnabled bool) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.sso_settings SET is_enabled = $1`, isEnabled)
	if err != nil {
		return fmt.Errorf("saving sso enabled flag: %w", err)
	}
	return nil
}

func (r *repository) disable(ctx context.Context) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.sso_settings SET is_enabled = FALSE, is_enforced = FALSE`)
	if err != nil {
		return fmt.Errorf("disabling sso: %w", err)
	}
	return nil
}

func (r *repository) setEnforced(ctx context.Context, isEnforced bool) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.sso_settings SET is_enforced = $1`, isEnforced)
	if err != nil {
		return fmt.Errorf("saving sso enforcement: %w", err)
	}
	return nil
}

func (r *repository) updateRules(ctx context.Context, allowedDomains []string, isAutoCreate bool, adminGroup string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.sso_settings SET allowed_domains = $1, is_auto_create = $2, admin_group = $3
	`, allowedDomains, isAutoCreate, adminGroup)
	if err != nil {
		return fmt.Errorf("saving sso rules: %w", err)
	}
	return nil
}

func (r *repository) saveMetadata(ctx context.Context, kind, issuer, metadataXml string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.sso_config SET issuer = $2, idp_metadata_xml = $3, idp_metadata_fetched_at = now() WHERE kind = $1
	`, kind, issuer, metadataXml)
	if err != nil {
		return fmt.Errorf("saving idp metadata: %w", err)
	}
	return nil
}

type storedKeyPair struct {
	Certificate string `db:"sp_cert"`
	KeyEnc      []byte `db:"sp_key_enc"`
	KeyNonce    []byte `db:"sp_key_nonce"`
}

func (r *repository) loadKeyPair(ctx context.Context) (storedKeyPair, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `SELECT sp_cert, sp_key_enc, sp_key_nonce FROM users.sso_settings`)
	if err != nil {
		return storedKeyPair{}, fmt.Errorf("querying sp key pair: %w", err)
	}
	stored, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[storedKeyPair])
	if err != nil {
		return storedKeyPair{}, fmt.Errorf("collecting sp key pair: %w", err)
	}
	return stored, nil
}

func (r *repository) saveKeyPair(ctx context.Context, stored storedKeyPair) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.sso_settings SET sp_cert = $1, sp_key_enc = $2, sp_key_nonce = $3
	`, stored.Certificate, stored.KeyEnc, stored.KeyNonce)
	if err != nil {
		return fmt.Errorf("saving sp key pair: %w", err)
	}
	return nil
}

// saveMissingKeyPair leaves a pair stored by a parallel request in place.
func (r *repository) saveMissingKeyPair(ctx context.Context, stored storedKeyPair) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.sso_settings SET sp_cert = $1, sp_key_enc = $2, sp_key_nonce = $3
		WHERE sp_cert = '' OR coalesce(length(sp_key_enc), 0) = 0
	`, stored.Certificate, stored.KeyEnc, stored.KeyNonce)
	if err != nil {
		return fmt.Errorf("saving missing sp key pair: %w", err)
	}
	return nil
}
