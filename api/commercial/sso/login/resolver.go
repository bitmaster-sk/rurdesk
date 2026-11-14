package login

import (
	"context"
	"errors"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/groups"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/avatar"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IResolver interface {
	Preview(ctx context.Context, id identity.Identity, rules Rules) (Outcome, error)
	Resolve(ctx context.Context, id identity.Identity, rules Rules) (Outcome, error)
}

type Resolver struct {
	pool       *pgxpool.Pool
	users      *repository.UserRepository
	identities *IdentityRepository
	groups     groups.IService
}

var _ IResolver = (*Resolver)(nil)

func NewResolver(pool *pgxpool.Pool, users *repository.UserRepository, groupSync groups.IService) *Resolver {
	return &Resolver{pool: pool, users: users, identities: NewIdentityRepository(pool), groups: groupSync}
}

func (r *Resolver) Preview(ctx context.Context, id identity.Identity, rules Rules) (Outcome, error) {
	candidates, err := r.candidates(ctx, id)
	if err != nil {
		return Outcome{}, err
	}
	return Decide(id, candidates, rules), nil
}

func (r *Resolver) Resolve(ctx context.Context, id identity.Identity, rules Rules) (Outcome, error) {
	var outcome Outcome
	var synced *groups.SyncResult
	err := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		var err error
		outcome, err = r.Preview(ctx, id, rules)
		if err != nil || outcome.Action == ActionReject {
			return err
		}
		user, err := r.apply(ctx, id, outcome)
		if err != nil {
			return err
		}
		outcome.User, err = r.syncProfile(ctx, id, user, rules)
		if err != nil {
			return err
		}
		synced, err = r.groups.Sync(ctx, outcome.User, id, rules.AdminGroup)
		return err
	})
	if err != nil {
		return Outcome{}, err
	}
	return outcome, r.groups.AfterSync(ctx, synced)
}

func (r *Resolver) candidates(ctx context.Context, id identity.Identity) (Candidates, error) {
	idUser, err := r.identities.LoadUserId(ctx, id.Issuer, id.Subject)
	if err == nil {
		user, err := r.users.LoadUser(ctx, idUser)
		if err != nil {
			return Candidates{}, err
		}
		return Candidates{Linked: user}, nil
	}
	if !errors.Is(err, ErrIdentityNotFound) {
		return Candidates{}, err
	}
	if id.Email == "" {
		return Candidates{}, nil
	}
	users, err := r.users.LoadUsersByEmailFold(ctx, id.Email)
	if err != nil {
		return Candidates{}, err
	}
	byEmail := make([]Candidate, 0, len(users))
	for _, user := range users {
		hasIssuer, err := r.identities.HasIssuer(ctx, user.IdUser, id.Issuer)
		if err != nil {
			return Candidates{}, err
		}
		byEmail = append(byEmail, Candidate{User: user, HasIssuerIdentity: hasIssuer})
	}
	return Candidates{ByEmail: byEmail}, nil
}

func (r *Resolver) apply(ctx context.Context, id identity.Identity, outcome Outcome) (*model.User, error) {
	switch outcome.Action {
	case ActionCreate:
		return r.create(ctx, id)
	case ActionLink:
		return outcome.User, r.identities.Insert(ctx, outcome.User.IdUser, id.Issuer, id.Subject)
	}
	return outcome.User, nil
}

func (r *Resolver) create(ctx context.Context, id identity.Identity) (*model.User, error) {
	user, err := r.users.InsertUser(ctx, &model.User{
		Name:          NewUserName(id),
		Email:         id.Email,
		ColorAvatarBg: avatar.RandomColor(),
	})
	if err != nil {
		return nil, err
	}
	if err := r.identities.Insert(ctx, user.IdUser, id.Issuer, id.Subject); err != nil {
		return nil, err
	}
	return r.users.LoadUser(ctx, user.IdUser)
}

func (r *Resolver) syncProfile(ctx context.Context, id identity.Identity, user *model.User, rules Rules) (*model.User, error) {
	isEmailTaken := false
	if id.Email != "" && id.Email != user.Email {
		var err error
		isEmailTaken, err = r.users.EmailTakenByOther(ctx, id.Email, user.IdUser)
		if err != nil {
			return nil, err
		}
	}
	profile := NextProfile(id, user, rules, isEmailTaken)
	if id.Email != "" && profile.Email != id.Email {
		extctx.GetLogger(ctx).Warn().Int64("idUser", user.IdUser).Msg("sso: e-mail from the identity provider was not applied")
	}
	if profile.Name == user.Name && profile.Email == user.Email {
		return user, nil
	}
	if err := r.users.UpdateProfile(ctx, user.IdUser, profile.Name, profile.Email); err != nil {
		return nil, err
	}
	user.Name, user.Email = profile.Name, profile.Email
	return user, nil
}
