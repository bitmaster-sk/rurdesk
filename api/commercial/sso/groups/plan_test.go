package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlanSync(t *testing.T) {
	isTrue, isFalse := true, false
	mappings := []Mapping{
		{GroupValue: "dev", IdTeam: 1},
		{GroupValue: "ops", IdTeam: 2},
		{GroupValue: "ops-oncall", IdTeam: 2},
		{GroupValue: "sales", IdTeam: 3},
	}
	tests := []struct {
		name     string
		input    SyncInput
		expected SyncPlan
	}{
		{
			name:     "adds and removes mapped teams",
			input:    SyncInput{Groups: []string{"dev"}, HasGroupsClaim: true, Mappings: mappings, MemberTeams: []int64{3}},
			expected: SyncPlan{AddTeams: []int64{1}, RemoveTeams: []int64{3}},
		},
		{
			name:     "a team stays when any of its values matches",
			input:    SyncInput{Groups: []string{"ops-oncall"}, HasGroupsClaim: true, Mappings: mappings, MemberTeams: []int64{2}},
			expected: SyncPlan{},
		},
		{
			name:     "unmapped teams are left alone",
			input:    SyncInput{Groups: []string{}, HasGroupsClaim: true, Mappings: mappings, MemberTeams: []int64{9}},
			expected: SyncPlan{},
		},
		{
			name:     "missing claim skips the sync",
			input:    SyncInput{Mappings: mappings, MemberTeams: []int64{1, 2}, AdminGroup: "admins", IsAdmin: true},
			expected: SyncPlan{},
		},
		{
			name:     "empty list removes every mapped membership",
			input:    SyncInput{Groups: []string{}, HasGroupsClaim: true, Mappings: mappings, MemberTeams: []int64{1, 2, 3}},
			expected: SyncPlan{RemoveTeams: []int64{1, 2, 3}},
		},
		{
			name:     "admin group grants admin",
			input:    SyncInput{Groups: []string{"admins"}, HasGroupsClaim: true, AdminGroup: "admins"},
			expected: SyncPlan{SetAdmin: &isTrue},
		},
		{
			name:     "admin group revokes admin",
			input:    SyncInput{Groups: []string{"dev"}, HasGroupsClaim: true, AdminGroup: "admins", IsAdmin: true},
			expected: SyncPlan{SetAdmin: &isFalse},
		},
		{
			name:     "without an admin group the flag is untouched",
			input:    SyncInput{Groups: []string{}, HasGroupsClaim: true, IsAdmin: true},
			expected: SyncPlan{},
		},
		{
			name:     "the last admin with a password keeps the flag",
			input:    SyncInput{Groups: []string{}, HasGroupsClaim: true, AdminGroup: "admins", IsAdmin: true, IsLastAdminWithPassword: true},
			expected: SyncPlan{IsAdminRemovalSkipped: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PlanSync(tt.input))
		})
	}
}
