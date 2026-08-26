package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent/skills"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillServiceSyncAndRestore(t *testing.T) {
	Setup(t)
	ctx := context.Background()
	repo := injector.GetSkillRepository()
	svc := injector.GetSkillService()

	require.NoError(t, svc.SyncBuiltins(ctx))
	all, err := svc.Load(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(all), 4, "builtins seeded")

	builtin := builtinByKey(t, all, "verification-rules")
	originalContent := builtin.Content

	_, err = repo.Update(ctx, builtin.IdSkill, builtin.Name, builtin.Description, "EDITED")
	require.NoError(t, err)
	require.NoError(t, svc.SyncBuiltins(ctx))
	reloaded, err := svc.LoadById(ctx, builtin.IdSkill)
	require.NoError(t, err)
	assert.Equal(t, "EDITED", reloaded.Content, "sync must never overwrite an edited row")
	assert.True(t, reloaded.IsEdited)

	restored, err := svc.Restore(ctx, builtin.IdSkill)
	require.NoError(t, err)
	assert.Equal(t, originalContent, restored.Content)
	assert.False(t, restored.IsEdited, "a restored builtin is untouched again")

	assert.ErrorIs(t, svc.Delete(ctx, builtin.IdSkill), errs.ErrSkillBuiltin)

	custom, err := repo.Insert(ctx, "My skill", "desc", "content")
	require.NoError(t, err)
	_, err = svc.Restore(ctx, custom.IdSkill)
	assert.ErrorIs(t, err, errs.ErrSkillNotBuiltin)
	require.NoError(t, svc.Delete(ctx, custom.IdSkill))

	_, err = svc.LoadById(ctx, custom.IdSkill)
	assert.ErrorIs(t, err, errs.ErrSkillNotFound, "a deleted skill is reported as missing, not as a nil row")

	_, err = svc.Restore(ctx, 999999)
	assert.ErrorIs(t, err, errs.ErrSkillNotFound)
}

// Makes a row look like it came from an older release: checksum matches content,
// so it counts as untouched.
func shipWithOlderVersion(t *testing.T, app *issue.Application, skill *model.Skill, content string) {
	t.Helper()
	_, err := app.Pool.Exec(context.Background(), `
		UPDATE agent.skill SET content = $2, builtin_checksum = $3 WHERE id_skill = $1
	`, skill.IdSkill, content, skills.Checksum(skill.Name, skill.Description, content))
	require.NoError(t, err)
}

func TestSkillServiceSyncUpdatesOnlyUntouchedBuiltins(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	repo := injector.GetSkillRepository()
	svc := injector.GetSkillService()

	require.NoError(t, svc.SyncBuiltins(ctx))
	all, err := svc.Load(ctx)
	require.NoError(t, err)
	untouched := builtinByKey(t, all, "verification-rules")
	edited := builtinByKey(t, all, "pr-rules")
	shippedContent := untouched.Content

	shipWithOlderVersion(t, app, untouched, "OLD SHIPPED TEXT")
	shipWithOlderVersion(t, app, edited, "OLD SHIPPED TEXT")
	_, err = repo.Update(ctx, edited.IdSkill, edited.Name, edited.Description, "HAND EDITED")
	require.NoError(t, err)

	require.NoError(t, svc.SyncBuiltins(ctx))

	reloaded, err := svc.LoadById(ctx, untouched.IdSkill)
	require.NoError(t, err)
	assert.Equal(t, shippedContent, reloaded.Content, "an untouched builtin follows the shipped version")
	require.NotNil(t, reloaded.BuiltinChecksum)
	assert.Equal(t, skills.Checksum(reloaded.Name, reloaded.Description, reloaded.Content), *reloaded.BuiltinChecksum)

	keptEdit, err := svc.LoadById(ctx, edited.IdSkill)
	require.NoError(t, err)
	assert.Equal(t, "HAND EDITED", keptEdit.Content, "a hand-edited builtin is never overwritten")

	_, err = svc.Restore(ctx, edited.IdSkill)
	require.NoError(t, err)
}

func TestSkillServiceSyncAdoptsRowWithoutChecksum(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	svc := injector.GetSkillService()

	require.NoError(t, svc.SyncBuiltins(ctx))
	all, err := svc.Load(ctx)
	require.NoError(t, err)
	untouched := builtinByKey(t, all, "repository-rules")

	_, err = app.Pool.Exec(ctx,
		`UPDATE agent.skill SET builtin_checksum = NULL WHERE id_skill = $1`, untouched.IdSkill)
	require.NoError(t, err)
	require.NoError(t, svc.SyncBuiltins(ctx))

	adopted, err := svc.LoadById(ctx, untouched.IdSkill)
	require.NoError(t, err)
	require.NotNil(t, adopted.BuiltinChecksum, "a row matching the shipped text is adopted as untouched")
	assert.Equal(t, untouched.Content, adopted.Content)

	_, err = app.Pool.Exec(ctx,
		`UPDATE agent.skill SET content = 'OLD EDIT', builtin_checksum = NULL WHERE id_skill = $1`, untouched.IdSkill)
	require.NoError(t, err)
	require.NoError(t, svc.SyncBuiltins(ctx))

	kept, err := svc.LoadById(ctx, untouched.IdSkill)
	require.NoError(t, err)
	assert.Nil(t, kept.BuiltinChecksum, "a row that differs from the shipped text stays unadopted")
	assert.Equal(t, "OLD EDIT", kept.Content)
	assert.True(t, kept.IsEdited, "an unadopted row that differs is reported as edited")

	_, err = svc.Restore(ctx, untouched.IdSkill)
	require.NoError(t, err)
}
