package model

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
)

// Guards against the phase vocabulary drifting from the state machine: every
// canonical phase must be mappable, and stale names must not.
func TestIsValidPhase(t *testing.T) {
	canonicalPhases := []string{
		constants.PhaseQueued,
		constants.PhaseInProgress,
		constants.PhaseAwaitingInput,
		constants.PhaseAwaitingApproval,
		constants.PhasePrOpen,
		constants.PhaseDone,
		constants.PhaseFailed,
		constants.PhaseCancelled,
	}

	tests := []struct {
		name    string
		phase   string
		isValid bool
	}{
		{name: "done is mappable so merged PRs can close the issue", phase: constants.PhaseDone, isValid: true},
		{name: "pr_open is mappable", phase: constants.PhasePrOpen, isValid: true},
		{name: "in_progress is mappable", phase: constants.PhaseInProgress, isValid: true},
		{name: "awaiting_input is mappable", phase: constants.PhaseAwaitingInput, isValid: true},
		{name: "stale name merged is rejected", phase: "merged", isValid: false},
		{name: "stale name pickup is rejected", phase: "pickup", isValid: false},
		{name: "stale name planning is rejected", phase: "planning", isValid: false},
		{name: "stale name implementing is rejected", phase: "implementing", isValid: false},
		{name: "unknown phase is rejected", phase: "nonexistent", isValid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.isValid, IsValidPhase(test.phase))
		})
	}

	for _, phase := range canonicalPhases {
		assert.Truef(t, IsValidPhase(phase), "canonical phase %q must be mappable", phase)
	}
}

func TestReplacePhaseStateMappingsValidate(t *testing.T) {
	idState := int64(7)

	tests := []struct {
		name     string
		mappings []PhaseStateMappingEntry
		wantErr  bool
	}{
		{
			name:     "done mapping is accepted",
			mappings: []PhaseStateMappingEntry{{Phase: constants.PhaseDone, IdState: &idState}},
			wantErr:  false,
		},
		{
			name:     "unknown phase is rejected",
			mappings: []PhaseStateMappingEntry{{Phase: "nonexistent", IdState: &idState}},
			wantErr:  true,
		},
		{
			name: "duplicate phase is rejected",
			mappings: []PhaseStateMappingEntry{
				{Phase: constants.PhaseDone, IdState: &idState},
				{Phase: constants.PhaseDone, IdState: &idState},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dto := ReplacePhaseStateMappingsReq{Mappings: test.mappings}
			err := dto.Validate()
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
