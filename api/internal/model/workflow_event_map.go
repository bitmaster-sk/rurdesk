package model

import (
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

// ValidWorkflowEvents lists events mappable to an issue state, sourced from the canonical
// constants so it can't drift from the state machine — workflow event lookups match
// the exact string TransitionPhase emits.
var ValidWorkflowEvents = []string{
	constants.PhaseQueued,
	constants.PhaseInProgress,
	constants.PhaseAwaitingInput,
	constants.PhaseAwaitingApproval,
	constants.PhasePrOpen,
	constants.PhaseDone,
	constants.PhaseFailed,
	constants.PhaseCancelled,
}

func IsValidWorkflowEvent(event string) bool {
	for _, validEvent := range ValidWorkflowEvents {
		if event == validEvent {
			return true
		}
	}
	return false
}

type WorkflowEventMapping struct {
	IdProject int64  `json:"idProject" db:"id_project"`
	Event     string `json:"event"     db:"event"`
	IdState   *int64 `json:"idState"   db:"id_state"`
}

type ReplaceWorkflowEventMappingsReq struct {
	Mappings []WorkflowEventMappingEntry `json:"mappings"`
}

type WorkflowEventMappingEntry struct {
	Event   string `json:"event"`
	IdState *int64 `json:"idState"`
}

func (dto *ReplaceWorkflowEventMappingsReq) Validate() error {
	seen := make(map[string]bool, len(dto.Mappings))
	for _, mapping := range dto.Mappings {
		if !IsValidWorkflowEvent(mapping.Event) {
			return errs.ErrValidation.WithMessage(fmt.Sprintf("unknown event: %s", mapping.Event))
		}
		if seen[mapping.Event] {
			return errs.ErrValidation.WithMessage(fmt.Sprintf("duplicate event: %s", mapping.Event))
		}
		seen[mapping.Event] = true
	}
	return nil
}
