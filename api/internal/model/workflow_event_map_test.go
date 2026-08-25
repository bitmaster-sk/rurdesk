package model

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
)

// Guards against the event vocabulary drifting from the state machine: every
// canonical phase must be mappable, and stale names must not.
func TestIsValidWorkflowEvent(t *testing.T) {
	canonicalEvents := []string{
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
		event   string
		isValid bool
	}{
		{name: "done is mappable so merged PRs can close the issue", event: constants.PhaseDone, isValid: true},
		{name: "pr_open is mappable", event: constants.PhasePrOpen, isValid: true},
		{name: "in_progress is mappable", event: constants.PhaseInProgress, isValid: true},
		{name: "awaiting_input is mappable", event: constants.PhaseAwaitingInput, isValid: true},
		{name: "stale name merged is rejected", event: "merged", isValid: false},
		{name: "stale name pickup is rejected", event: "pickup", isValid: false},
		{name: "stale name planning is rejected", event: "planning", isValid: false},
		{name: "stale name implementing is rejected", event: "implementing", isValid: false},
		{name: "unknown event is rejected", event: "nonexistent", isValid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.isValid, IsValidWorkflowEvent(test.event))
		})
	}

	for _, event := range canonicalEvents {
		assert.Truef(t, IsValidWorkflowEvent(event), "canonical event %q must be mappable", event)
	}
}

func TestReplaceWorkflowEventMappingsValidate(t *testing.T) {
	idState := int64(7)

	tests := []struct {
		name     string
		mappings []WorkflowEventMappingEntry
		wantErr  bool
	}{
		{
			name:     "done mapping is accepted",
			mappings: []WorkflowEventMappingEntry{{Event: constants.PhaseDone, IdState: &idState}},
			wantErr:  false,
		},
		{
			name:     "unknown event is rejected",
			mappings: []WorkflowEventMappingEntry{{Event: "nonexistent", IdState: &idState}},
			wantErr:  true,
		},
		{
			name: "duplicate event is rejected",
			mappings: []WorkflowEventMappingEntry{
				{Event: constants.PhaseDone, IdState: &idState},
				{Event: constants.PhaseDone, IdState: &idState},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dto := ReplaceWorkflowEventMappingsReq{Mappings: test.mappings}
			err := dto.Validate()
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
