package authpolicy

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type PasswordLoginPolicy interface {
	Allow(ctx context.Context, user *model.User) bool
}

type ChangeKind string

const (
	ChangeDeactivate       ChangeKind = "deactivate"
	ChangeDelete           ChangeKind = "delete"
	ChangeRemoveAdmin      ChangeKind = "remove_admin"
	ChangeRemoveBreakGlass ChangeKind = "remove_break_glass"
)

type UserChange struct {
	IdUser int64
	Kind   ChangeKind
}

type UserChangeGuard interface {
	Check(ctx context.Context, change UserChange) error
}

type AuthMethodsProvider interface {
	Methods(ctx context.Context) model.AuthMethodsRes
}

type Free struct{}

func (Free) Allow(context.Context, *model.User) bool { return true }

func (Free) Check(context.Context, UserChange) error { return nil }

func (Free) Methods(context.Context) model.AuthMethodsRes {
	return model.AuthMethodsRes{Password: true}
}
