package service

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent/skills"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFillFlags(t *testing.T) {
	svc := NewSkillService(nil, nil)

	shipped, ok := skills.BuiltinByKey("verification-rules")
	require.True(t, ok)

	builtinSkill := func(content string, checksum *string) *model.Skill {
		key := shipped.Key
		return &model.Skill{
			Name:            shipped.Name,
			Description:     shipped.Description,
			Content:         content,
			BuiltinKey:      &key,
			BuiltinChecksum: checksum,
		}
	}
	checksumOf := func(content string) *string {
		sum := skills.Checksum(shipped.Name, shipped.Description, content)
		return &sum
	}
	unknownKey := "gone-from-this-build"

	tests := []struct {
		name      string
		skill     *model.Skill
		isBuiltin bool
		isEdited  bool
	}{
		{
			name:  "custom skill is never builtin nor edited",
			skill: &model.Skill{Name: "Mine", Content: "anything"},
		},
		{
			name:      "builtin matching its checksum is untouched",
			skill:     builtinSkill(shipped.Content, checksumOf(shipped.Content)),
			isBuiltin: true,
		},
		{
			name:      "builtin differing from its checksum was hand-edited",
			skill:     builtinSkill("HAND EDITED", checksumOf(shipped.Content)),
			isBuiltin: true,
			isEdited:  true,
		},
		{
			name:      "untouched row from an older release is not edited",
			skill:     builtinSkill("OLD SHIPPED TEXT", checksumOf("OLD SHIPPED TEXT")),
			isBuiltin: true,
		},
		{
			name:      "without a checksum, matching the shipped text counts as untouched",
			skill:     builtinSkill(shipped.Content, nil),
			isBuiltin: true,
		},
		{
			name:      "without a checksum, differing from the shipped text counts as edited",
			skill:     builtinSkill("SOMETHING ELSE", nil),
			isBuiltin: true,
			isEdited:  true,
		},
		{
			name:      "a builtin no longer shipped cannot be judged edited",
			skill:     &model.Skill{Name: "Gone", Content: "x", BuiltinKey: &unknownKey},
			isBuiltin: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc.fillFlags(tc.skill)

			assert.Equal(t, tc.isBuiltin, tc.skill.IsBuiltin)
			assert.Equal(t, tc.isEdited, tc.skill.IsEdited)
		})
	}
}

func TestFillFlagsSkipsNilEntries(t *testing.T) {
	svc := NewSkillService(nil, nil)

	skill := &model.Skill{Name: "Mine", Content: "x"}

	assert.NotPanics(t, func() { svc.fillFlags(nil, skill, nil) })
	assert.False(t, skill.IsBuiltin)
}
