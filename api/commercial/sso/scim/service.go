package scim

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/avatar"
	"github.com/bitmaster-sk/rurdesk/api/internal/deactivation"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type State struct {
	IsEnabled     bool       `json:"isEnabled"`
	HasToken      bool       `json:"hasToken"`
	TokenCreateAt *time.Time `json:"tokenCreateAt"`
	LastUsedAt    *time.Time `json:"lastUsedAt"`
	BaseUrl       string     `json:"baseUrl"`
}

type Service struct {
	pool         *pgxpool.Pool
	repo         *Repository
	users        *repository.UserRepository
	deactivation *deactivation.Service
	config       config.IService
}

func NewService(pool *pgxpool.Pool, users *repository.UserRepository, deactivationService *deactivation.Service, configService config.IService) *Service {
	return &Service{pool: pool, repo: NewRepository(pool), users: users, deactivation: deactivationService, config: configService}
}

func (s *Service) Authenticate(ctx context.Context, token string) error {
	stored, err := s.repo.loadToken(ctx)
	if err != nil {
		return err
	}
	if !stored.IsEnabled || !isTokenValid(token, stored.Hash) {
		return errUnauthorized
	}
	return s.repo.touchToken(ctx)
}

func (s *Service) IsProvisioning(ctx context.Context) (bool, error) {
	return s.repo.IsProvisioning(ctx)
}

func (s *Service) State(ctx context.Context) (State, error) {
	stored, err := s.repo.loadToken(ctx)
	if err != nil {
		return State{}, err
	}
	info, err := s.repo.loadTokenInfo(ctx)
	if err != nil {
		return State{}, err
	}
	state := State{IsEnabled: stored.IsEnabled, HasToken: info != nil}
	if info != nil {
		state.TokenCreateAt, state.LastUsedAt = &info.CreateAt, info.LastUsedAt
	}
	if baseUrl, err := publicurl.Load(); err == nil && baseUrl != "" {
		state.BaseUrl = baseUrl + BasePath
	}
	return state, nil
}

func (s *Service) SetEnabled(ctx context.Context, isEnabled bool) error {
	return s.repo.setEnabled(ctx, isEnabled)
}

func (s *Service) GenerateToken(ctx context.Context, idUser int64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	return token, s.repo.saveToken(ctx, hashToken(token), idUser)
}

func (s *Service) DeleteToken(ctx context.Context) error {
	return s.repo.deleteToken(ctx)
}

func (s *Service) List(ctx context.Context, filter *Filter, page Page) (ListResponse, error) {
	accounts, total, err := s.repo.list(ctx, filter, page)
	if err != nil {
		return ListResponse{}, err
	}
	resources := make([]UserResource, 0, len(accounts))
	for _, account := range accounts {
		resources = append(resources, resourceOf(account))
	}
	return ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: total,
		StartIndex:   page.StartIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

func (s *Service) Get(ctx context.Context, idUser int64) (UserResource, error) {
	account, err := s.repo.loadVisible(ctx, idUser)
	if err != nil {
		return UserResource{}, err
	}
	return resourceOf(*account), nil
}

func (s *Service) Create(ctx context.Context, changes Changes) (UserResource, error) {
	address, ok := changes.Email()
	if !ok {
		return UserResource{}, invalidValue("userName or emails is required")
	}
	if err := s.checkDomain(ctx, address); err != nil {
		return UserResource{}, err
	}
	var idUser int64
	var result *deactivation.Result
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		candidates, err := s.repo.findByEmail(ctx, address)
		if err != nil {
			return err
		}
		switch len(candidates) {
		case 0:
			idUser, err = s.insert(ctx, address, changes)
			return err
		case 1:
			idUser = candidates[0].IdUser
			result, err = s.takeOver(ctx, candidates[0], changes)
			return err
		}
		return uniqueness("more accounts use this e-mail")
	})
	if err != nil {
		return UserResource{}, mapError(err)
	}
	if err := s.deactivation.AfterCommit(ctx, result); err != nil {
		return UserResource{}, err
	}
	return s.Get(ctx, idUser)
}

func (s *Service) insert(ctx context.Context, address string, changes Changes) (int64, error) {
	externalId := normalizedExternalId(changes.ExternalId)
	if err := s.checkExternalId(ctx, externalId, 0); err != nil {
		return 0, err
	}
	name, ok := changes.Name()
	if !ok {
		name, _, _ = strings.Cut(address, "@")
	}
	user, err := s.users.InsertUser(ctx, &model.User{Name: name, Email: address, ColorAvatarBg: avatar.RandomColor()})
	if err != nil {
		return 0, err
	}
	if err := s.repo.setExternalId(ctx, user.IdUser, externalId); err != nil {
		return 0, err
	}
	if changes.IsDeactivation() {
		return user.IdUser, s.users.SetActive(ctx, user.IdUser, false)
	}
	return user.IdUser, nil
}

func (s *Service) takeOver(ctx context.Context, existing candidate, changes Changes) (*deactivation.Result, error) {
	if existing.IsAgent || existing.IsBreakGlass {
		return nil, uniqueness("the e-mail belongs to an account SCIM does not manage")
	}
	externalId := normalizedExternalId(changes.ExternalId)
	if existing.ExternalId != nil && (externalId == nil || *existing.ExternalId != *externalId) {
		return nil, uniqueness("the e-mail belongs to an account with another externalId")
	}
	if err := s.checkExternalId(ctx, externalId, existing.IdUser); err != nil {
		return nil, err
	}
	if err := s.repo.setDeleted(ctx, existing.IdUser, false); err != nil {
		return nil, err
	}
	if err := s.repo.setExternalId(ctx, existing.IdUser, externalId); err != nil {
		return nil, err
	}
	account, err := s.repo.loadVisible(ctx, existing.IdUser)
	if err != nil {
		return nil, err
	}
	if err := s.updateProfile(ctx, account, account.Email, changes); err != nil {
		return nil, err
	}
	isActive := changes.Active == nil || *changes.Active
	return s.deactivation.ApplyInTx(ctx, existing.IdUser, isActive, deactivation.Options{SkipLastOwnerGuard: true})
}

// Update serves PUT and PATCH. Without a license only a deactivation goes through, everything else in the request is ignored.
func (s *Service) Update(ctx context.Context, idUser int64, changes Changes, isLicensed bool) (UserResource, error) {
	if !isLicensed && !changes.IsDeactivation() {
		return UserResource{}, errForbidden
	}
	var result *deactivation.Result
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		account, err := s.repo.loadVisible(ctx, idUser)
		if err != nil {
			return err
		}
		if isLicensed {
			if err := s.applyLicensed(ctx, account, changes); err != nil {
				return err
			}
		}
		if changes.Active == nil {
			return nil
		}
		result, err = s.deactivation.ApplyInTx(ctx, idUser, *changes.Active, deactivation.Options{SkipLastOwnerGuard: true})
		return err
	})
	if err != nil {
		return UserResource{}, mapError(err)
	}
	if err := s.deactivation.AfterCommit(ctx, result); err != nil {
		return UserResource{}, err
	}
	return s.Get(ctx, idUser)
}

func (s *Service) applyLicensed(ctx context.Context, account *Account, changes Changes) error {
	address := account.Email
	if requested, ok := changes.Email(); ok && requested != account.Email {
		if err := s.checkDomain(ctx, requested); err != nil {
			return err
		}
		isTaken, err := s.users.EmailTakenByOther(ctx, requested, account.IdUser)
		if err != nil {
			return err
		}
		if isTaken {
			return uniqueness("another account uses this e-mail")
		}
		address = requested
	}
	if err := s.updateProfile(ctx, account, address, changes); err != nil {
		return err
	}
	if changes.ExternalId == nil {
		return nil
	}
	externalId := normalizedExternalId(changes.ExternalId)
	if err := s.checkExternalId(ctx, externalId, account.IdUser); err != nil {
		return err
	}
	return s.repo.setExternalId(ctx, account.IdUser, externalId)
}

func (s *Service) updateProfile(ctx context.Context, account *Account, address string, changes Changes) error {
	name := account.Name
	if requested, ok := changes.Name(); ok {
		name = requested
	}
	if name == account.Name && address == account.Email {
		return nil
	}
	return s.users.UpdateProfile(ctx, account.IdUser, name, address)
}

func (s *Service) Delete(ctx context.Context, idUser int64) error {
	var result *deactivation.Result
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if _, err := s.repo.loadVisible(ctx, idUser); err != nil {
			return err
		}
		var err error
		result, err = s.deactivation.ApplyInTx(ctx, idUser, false, deactivation.Options{SkipLastOwnerGuard: true})
		if err != nil {
			return err
		}
		return s.repo.setDeleted(ctx, idUser, true)
	})
	if err != nil {
		return mapError(err)
	}
	return s.deactivation.AfterCommit(ctx, result)
}

func (s *Service) checkDomain(ctx context.Context, address string) error {
	settings, err := s.config.Settings(ctx)
	if err != nil {
		return err
	}
	_, domain, _ := strings.Cut(address, "@")
	if domain == "" || !slices.Contains(settings.AllowedDomains, domain) {
		return invalidValue("the e-mail domain is not in the allowed domains")
	}
	return nil
}

func (s *Service) checkExternalId(ctx context.Context, externalId *string, idUser int64) error {
	if externalId == nil {
		return nil
	}
	isTaken, err := s.repo.externalIdTakenByOther(ctx, *externalId, idUser)
	if err != nil {
		return err
	}
	if isTaken {
		return uniqueness("another account uses this externalId")
	}
	return nil
}

func normalizedExternalId(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapError(err error) error {
	if errors.Is(err, errs.ErrLastAdmin) {
		return errLastAdmin
	}
	if errors.Is(err, repository.ErrEmailTaken) {
		return uniqueness("another account uses this e-mail")
	}
	return err
}

func resourceOf(account Account) UserResource {
	id := strconv.FormatInt(account.IdUser, 10)
	location := BasePath + "/Users/" + id
	if baseUrl, err := publicurl.Load(); err == nil && baseUrl != "" {
		location = baseUrl + location
	}
	resource := UserResource{
		Schemas:     []string{SchemaUser},
		Id:          id,
		UserName:    account.Email,
		Name:        Name{Formatted: account.Name},
		DisplayName: account.Name,
		Emails:      []Email{{Value: account.Email, Type: "work", Primary: true}},
		Active:      account.IsActive,
		Meta:        Meta{ResourceType: "User", Location: location},
	}
	if account.ExternalId != nil {
		resource.ExternalId = *account.ExternalId
	}
	return resource
}
