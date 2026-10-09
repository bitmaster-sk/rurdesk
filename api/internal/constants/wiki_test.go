package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWikiProposalStateOf(t *testing.T) {
	cases := []struct {
		name     string
		decision WikiProposalDecision
		phase    string
		want     WikiProposalState
	}{
		{"open while the agent works", WikiProposalOpen, PhaseInProgress, WikiProposalStateOpen},
		{"open while the pull request is open", WikiProposalOpen, PhasePrOpen, WikiProposalStateOpen},
		{"open after the merge", WikiProposalOpen, PhaseDone, WikiProposalReady},
		{"open after a failed run", WikiProposalOpen, PhaseFailed, WikiProposalDiscarded},
		{"open after a cancelled run", WikiProposalOpen, PhaseCancelled, WikiProposalDiscarded},
		{"approved while the pull request is open", WikiProposalApproved, PhasePrOpen, WikiProposalStateApproved},
		{"approved but the pull request was closed", WikiProposalApproved, PhaseCancelled, WikiProposalDiscarded},
		{"needs resolving after the merge", WikiProposalNeedsResolving, PhaseDone, WikiProposalStateNeedsResolving},
		{"accepted stays accepted", WikiProposalAccepted, PhaseDone, WikiProposalStateAccepted},
		{"rejected stays rejected even if the run is revived", WikiProposalRejected, PhasePrOpen, WikiProposalStateRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, WikiProposalStateOf(tc.decision, tc.phase))
		})
	}
}
