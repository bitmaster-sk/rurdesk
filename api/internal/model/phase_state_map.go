package model

import (
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// ValidPhases lists phases mappable to an issue state, sourced from the canonical
// constants so it can't drift from the state machine — phase_state lookups match
// the exact string TransitionPhase emits.
var ValidPhases = []string{
	constants.PhaseQueued,
	constants.PhaseInProgress,
	constants.PhaseAwaitingInput,
	constants.PhaseAwaitingApproval,
	constants.PhasePrOpen,
	constants.PhaseDone,
	constants.PhaseFailed,
	constants.PhaseCancelled,
}

func IsValidPhase(phase string) bool {
	for _, validPhase := range ValidPhases {
		if phase == validPhase {
			return true
		}
	}
	return false
}

type PhaseStateMapping struct {
	IdProject int64  `json:"idProject" db:"id_project"`
	Phase     string `json:"phase"     db:"phase"`
	IdState   *int64 `json:"idState"   db:"id_state"`
}

type ReplacePhaseStateMappingsReq struct {
	Mappings []PhaseStateMappingEntry `json:"mappings"`
}

type PhaseStateMappingEntry struct {
	Phase   string `json:"phase"`
	IdState *int64 `json:"idState"`
}

func (dto *ReplacePhaseStateMappingsReq) Validate() error {
	seen := make(map[string]bool, len(dto.Mappings))
	for _, mapping := range dto.Mappings {
		if !IsValidPhase(mapping.Phase) {
			return fmt.Errorf("unknown phase: %s", mapping.Phase)
		}
		if seen[mapping.Phase] {
			return fmt.Errorf("duplicate phase: %s", mapping.Phase)
		}
		seen[mapping.Phase] = true
	}
	return nil
}
