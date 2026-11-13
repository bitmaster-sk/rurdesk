package config

import (
	"context"
	"errors"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IService interface {
	LoadState(ctx context.Context) (*StateRes, error)
	SaveDraft(ctx context.Context, idUser int64, req SaveDraftReq) error
	DraftForTest(ctx context.Context) (Connection, string, error)
	MarkTested(ctx context.Context, hash string) error
	Activate(ctx context.Context, idUser int64) error
	Disable(ctx context.Context) error
	SetEnforced(ctx context.Context, isEnforced bool) error
	SaveMetadata(ctx context.Context, isActive bool, metadataXml string) error
	KeyPair(ctx context.Context) (*KeyPair, error)
	RegenerateKeyPair(ctx context.Context) error
	UpdateRules(ctx context.Context, req UpdateRulesReq) error
	Active(ctx context.Context) (Connection, error)
	Settings(ctx context.Context) (Settings, error)
}

type Service struct {
	pool *pgxpool.Pool
	repo *repository
}

var _ IService = (*Service)(nil)

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, repo: &repository{pool: pool}}
}

func ReadyBaseUrl() (string, error) {
	baseUrl, err := publicurl.Load()
	if err != nil {
		return "", ErrPublicUrlInvalid
	}
	if err := ValidatePublicBaseUrl(baseUrl); err != nil {
		return "", err
	}
	return baseUrl, nil
}

type loaded struct {
	connection Connection
	stored     *storedConnection
	hash       string
}

func (s *Service) load(ctx context.Context, kind string) (*loaded, error) {
	key, err := LoadKey()
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.loadConnection(ctx, kind)
	if err != nil {
		return nil, err
	}
	secret, err := openStoredSecret(key, stored)
	if err != nil {
		return nil, err
	}
	connection := stored.connection(secret)
	hash, err := Hash(key, connection)
	if err != nil {
		return nil, err
	}
	return &loaded{connection: connection, stored: stored, hash: hash}, nil
}

func (s *Service) SaveDraft(ctx context.Context, idUser int64, req SaveDraftReq) error {
	key, err := LoadKey()
	if err != nil {
		return err
	}
	existing, err := s.repo.loadConnection(ctx, kindDraft)
	if errors.Is(err, ErrNotConfigured) {
		existing = nil
	} else if err != nil {
		return err
	}
	var stored *storedConnection
	if req.Protocol == ProtocolSaml {
		stored, err = s.samlDraft(ctx, key, req, existing)
	} else {
		stored, err = oidcDraft(key, req, existing)
	}
	if err != nil {
		return err
	}
	return s.repo.saveDraft(ctx, idUser, stored)
}

func oidcDraft(key []byte, req SaveDraftReq, existing *storedConnection) (*storedConnection, error) {
	if strings.TrimSpace(req.ClientId) == "" {
		return nil, ErrClientIdRequired
	}
	issuer, err := ValidateIssuer(req.Issuer)
	if err != nil {
		return nil, err
	}
	stored := &storedConnection{
		Protocol:       ProtocolOidc,
		Issuer:         issuer,
		ClientId:       strings.TrimSpace(req.ClientId),
		HasGroupsScope: req.HasGroupsScope,
		EmailClaim:     claimOrDefault(req.EmailClaim, DefaultEmailClaim),
		GroupsClaim:    claimOrDefault(req.GroupsClaim, DefaultGroupsClaim),
	}
	if req.ClientSecret != "" {
		stored.ClientSecretEnc, stored.ClientSecretNonce, err = sealSecret(key, req.ClientSecret)
		return stored, err
	}
	if existing == nil || len(existing.ClientSecretEnc) == 0 {
		return nil, ErrSecretRequired
	}
	stored.ClientSecretEnc, stored.ClientSecretNonce = existing.ClientSecretEnc, existing.ClientSecretNonce
	return stored, nil
}

func (s *Service) samlDraft(ctx context.Context, key []byte, req SaveDraftReq, existing *storedConnection) (*storedConnection, error) {
	stored := &storedConnection{
		Protocol:         ProtocolSaml,
		EmailClaim:       claimOrDefault(req.EmailClaim, DefaultEmailClaim),
		GroupsClaim:      claimOrDefault(req.GroupsClaim, DefaultGroupsClaim),
		NameAttribute:    claimOrDefault(req.NameAttribute, DefaultNameAttribute),
		SubjectAttribute: strings.TrimSpace(req.SubjectAttribute),
	}
	switch {
	case strings.TrimSpace(req.MetadataUrl) != "" && strings.TrimSpace(req.MetadataXml) != "":
		return nil, ErrMetadataAmbiguous
	case strings.TrimSpace(req.MetadataUrl) != "":
		metadataUrl, err := ValidateMetadataUrl(req.MetadataUrl)
		if err != nil {
			return nil, err
		}
		stored.MetadataUrl = metadataUrl
		if existing != nil && existing.Protocol == ProtocolSaml && existing.MetadataUrl == metadataUrl {
			stored.Issuer, stored.MetadataXml, stored.MetadataFetchedAt = existing.Issuer, existing.MetadataXml, existing.MetadataFetchedAt
		}
	case strings.TrimSpace(req.MetadataXml) != "":
		descriptor, err := ParseIdpMetadata(req.MetadataXml)
		if err != nil {
			return nil, err
		}
		stored.Issuer, stored.MetadataXml = descriptor.EntityID, req.MetadataXml
	case existing != nil && existing.Protocol == ProtocolSaml && existing.MetadataUrl == "" && existing.MetadataXml != "":
		stored.Issuer, stored.MetadataXml = existing.Issuer, existing.MetadataXml
	default:
		return nil, ErrMetadataRequired
	}
	return stored, s.ensureKeyPair(ctx, key)
}

func (s *Service) SaveMetadata(ctx context.Context, isActive bool, metadataXml string) error {
	descriptor, err := ParseIdpMetadata(metadataXml)
	if err != nil {
		return err
	}
	kind := kindDraft
	if isActive {
		kind = kindActive
	}
	return s.repo.saveMetadata(ctx, kind, descriptor.EntityID, metadataXml)
}

func (s *Service) DraftForTest(ctx context.Context) (Connection, string, error) {
	draft, err := s.load(ctx, kindDraft)
	if err != nil {
		return Connection{}, "", err
	}
	return draft.connection, draft.hash, nil
}

func (s *Service) MarkTested(ctx context.Context, hash string) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.repo.lockDraft(ctx); err != nil {
			return err
		}
		draft, err := s.load(ctx, kindDraft)
		if err != nil {
			return err
		}
		if !matchesHash(&draft.hash, hash) {
			return ErrDraftChanged
		}
		return s.repo.setTestedHash(ctx, hash)
	})
}

func (s *Service) Activate(ctx context.Context, idUser int64) error {
	if _, err := ReadyBaseUrl(); err != nil {
		return err
	}
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.repo.lockDraft(ctx); err != nil {
			return err
		}
		draft, err := s.load(ctx, kindDraft)
		if err != nil {
			return err
		}
		if !matchesHash(draft.stored.TestedHash, draft.hash) {
			return ErrNotTested
		}
		if err := s.repo.copyDraftToActive(ctx, idUser); err != nil {
			return err
		}
		return s.repo.setEnabled(ctx, true)
	})
}

func (s *Service) Disable(ctx context.Context) error {
	return s.repo.disable(ctx)
}

func (s *Service) SetEnforced(ctx context.Context, isEnforced bool) error {
	return s.repo.setEnforced(ctx, isEnforced)
}

func (s *Service) UpdateRules(ctx context.Context, req UpdateRulesReq) error {
	domains, err := NormalizeDomains(req.AllowedDomains)
	if err != nil {
		return err
	}
	return s.repo.updateRules(ctx, domains, req.IsAutoCreate, strings.TrimSpace(req.AdminGroup))
}

func (s *Service) Active(ctx context.Context) (Connection, error) {
	active, err := s.load(ctx, kindActive)
	if err != nil {
		return Connection{}, err
	}
	return active.connection, nil
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.repo.loadSettings(ctx)
}

func (s *Service) LoadState(ctx context.Context) (*StateRes, error) {
	settings, err := s.repo.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	key, keyErr := LoadKey()
	draft, err := s.describe(ctx, kindDraft, key)
	if err != nil {
		return nil, err
	}
	active, err := s.describe(ctx, kindActive, key)
	if err != nil {
		return nil, err
	}
	state := &StateRes{
		HasEncryptionKey:      keyErr == nil,
		IsSecretUndecryptable: draft.isUndecryptable || active.isUndecryptable,
		Draft:                 draft.response,
		Active:                active.response,
		IsDraftTested:         draft.isTested,
		IsDraftActive:         draft.hash != "" && draft.hash == active.hash,
		IsEnabled:             settings.IsEnabled,
		Rules:                 RulesRes{AllowedDomains: settings.AllowedDomains, IsAutoCreate: settings.IsAutoCreate, AdminGroup: settings.AdminGroup},
		IsEnforced:            settings.IsEnforced,
	}
	if baseUrl, err := publicurl.Load(); err == nil {
		state.PublicBaseUrl = baseUrl
	}
	if baseUrl, err := ReadyBaseUrl(); err == nil {
		state.IsPublicBaseUrlValid = true
		state.RedirectUri = baseUrl + CallbackPath
		state.AcsUrl = baseUrl + AcsPath
		state.SpEntityId = baseUrl + SpMetadataPath
	}
	if key != nil && state.IsPublicBaseUrlValid {
		// The settings page shows the SP metadata URL from here on, so its certificate must exist.
		if err := s.ensureKeyPair(ctx, key); err != nil && !errors.Is(err, ErrSpKeyUndecryptable) {
			return nil, err
		}
	}
	if key != nil {
		stored, err := s.repo.loadKeyPair(ctx)
		if err != nil {
			return nil, err
		}
		_, err = openKeyPair(key, stored)
		state.IsSpKeyUndecryptable = errors.Is(err, ErrSpKeyUndecryptable)
	}
	return state, nil
}

type described struct {
	response        *ConnectionRes
	isUndecryptable bool
	isTested        bool
	hash            string
}

func (s *Service) describe(ctx context.Context, kind string, key []byte) (*described, error) {
	stored, err := s.repo.loadConnection(ctx, kind)
	if errors.Is(err, ErrNotConfigured) {
		return &described{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := &described{response: stored.response()}
	if key == nil {
		return result, nil
	}
	secret, err := openStoredSecret(key, stored)
	if err != nil {
		result.isUndecryptable = true
		return result, nil
	}
	hash, err := Hash(key, stored.connection(secret))
	if err != nil {
		return nil, err
	}
	result.isTested = matchesHash(stored.TestedHash, hash)
	result.hash = hash
	return result, nil
}

func claimOrDefault(claim, fallback string) string {
	if trimmed := strings.TrimSpace(claim); trimmed != "" {
		return trimmed
	}
	return fallback
}

func openStoredSecret(key []byte, stored *storedConnection) (string, error) {
	if len(stored.ClientSecretEnc) == 0 {
		return "", nil
	}
	return openSecret(key, stored.ClientSecretEnc, stored.ClientSecretNonce)
}
