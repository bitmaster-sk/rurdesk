package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWikiProposalStateOf(t *testing.T) {
	cases := []struct {
		name     string
		decision WikiProposalDecision
		phase    *string
		want     WikiProposalState
	}{
		{"open while the agent works", WikiProposalOpen, phase(PhaseInProgress), WikiProposalStateOpen},
		{"open while the pull request is open", WikiProposalOpen, phase(PhasePrOpen), WikiProposalStateOpen},
		{"open after the merge", WikiProposalOpen, phase(PhaseDone), WikiProposalReady},
		{"open after a failed run", WikiProposalOpen, phase(PhaseFailed), WikiProposalDiscarded},
		{"open after a cancelled run", WikiProposalOpen, phase(PhaseCancelled), WikiProposalDiscarded},
		{"approved while the pull request is open", WikiProposalApproved, phase(PhasePrOpen), WikiProposalStateApproved},
		{"approved but the pull request was closed", WikiProposalApproved, phase(PhaseCancelled), WikiProposalDiscarded},
		{"needs resolving after the merge", WikiProposalNeedsResolving, phase(PhaseDone), WikiProposalStateNeedsResolving},
		{"accepted stays accepted", WikiProposalAccepted, phase(PhaseDone), WikiProposalStateAccepted},
		{"rejected stays rejected even if the run is revived", WikiProposalRejected, phase(PhasePrOpen), WikiProposalStateRejected},
		{"without a run it is ready at once", WikiProposalOpen, nil, WikiProposalReady},
		{"without a run accepted stays accepted", WikiProposalAccepted, nil, WikiProposalStateAccepted},
		{"without a run rejected stays rejected", WikiProposalRejected, nil, WikiProposalStateRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, WikiProposalStateOf(tc.decision, tc.phase))
		})
	}
}

func phase(value string) *string {
	return &value
}
