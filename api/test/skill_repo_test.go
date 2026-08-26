package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtinByKey(t *testing.T, all []*model.Skill, key string) *model.Skill {
	t.Helper()
	for _, s := range all {
		if s.BuiltinKey != nil && *s.BuiltinKey == key {
			return s
		}
	}
	t.Fatalf("builtin skill %q not seeded", key)
	return nil
}

func TestSkillRepositoryDuplicateName(t *testing.T) {
	Setup(t)
	ctx := context.Background()
	repo := injector.GetSkillRepository()

	first, err := repo.Insert(ctx, "Dup name", "d", "c")
	require.NoError(t, err)
	defer repo.Delete(ctx, first.IdSkill) //nolint:errcheck

	_, err = repo.Insert(ctx, "Dup name", "d", "c")
	assert.ErrorIs(t, err, errs.ErrSkillNameTaken)
}

func TestSkillRepositoryDeleteReportsMissingRow(t *testing.T) {
	Setup(t)

	assert.ErrorIs(t, injector.GetSkillRepository().Delete(context.Background(), 999999),
		errs.ErrSkillNotFound)
}

func TestSkillRepositoryLoadByIds(t *testing.T) {
	Setup(t)
	ctx := context.Background()
	repo := injector.GetSkillRepository()

	require.NoError(t, injector.GetSkillService().SyncBuiltins(ctx))
	all, err := repo.Load(ctx)
	require.NoError(t, err)
	first := builtinByKey(t, all, "verification-rules")

	loaded, err := repo.LoadByIds(ctx, []int64{first.IdSkill, 999999})
	require.NoError(t, err)
	require.Len(t, loaded, 1, "unknown ids are simply absent")
	assert.Equal(t, first.IdSkill, loaded[0].IdSkill)

	none, err := repo.LoadByIds(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestSkillRepositoryLoadByBuiltinKey(t *testing.T) {
	Setup(t)
	ctx := context.Background()
	repo := injector.GetSkillRepository()

	require.NoError(t, injector.GetSkillService().SyncBuiltins(ctx))

	found, err := repo.LoadByBuiltinKey(ctx, "verification-rules")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "verification-rules", *found.BuiltinKey)

	_, err = repo.LoadByBuiltinKey(ctx, "no-such-key")
	assert.ErrorIs(t, err, errs.ErrSkillNotFound, "absence is reported the same way by every loader")
}
