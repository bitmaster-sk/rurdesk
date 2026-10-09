package constants

type WikiSpaceKind string

const (
	WikiSpaceInstance WikiSpaceKind = "instance"
	WikiSpaceProject  WikiSpaceKind = "project"
)

type WikiAgentAccess string

const (
	WikiAgentAccessAlways   WikiAgentAccess = "always"
	WikiAgentAccessOnDemand WikiAgentAccess = "on_demand"
	WikiAgentAccessHidden   WikiAgentAccess = "hidden"
)

type WikiIssueLinkSource string

const (
	WikiIssueLinkDescription WikiIssueLinkSource = "description"
	WikiIssueLinkManual      WikiIssueLinkSource = "manual"
)

const WikiTrashRetentionDays = 30

func IsValidWikiAgentAccess(access WikiAgentAccess) bool {
	switch access {
	case WikiAgentAccessAlways, WikiAgentAccessOnDemand, WikiAgentAccessHidden:
		return true
	default:
		return false
	}
}

type WikiReadSource string

const (
	WikiReadPromptAlways WikiReadSource = "prompt_always"
	WikiReadPromptLinked WikiReadSource = "prompt_linked"
	WikiReadPromptIndex  WikiReadSource = "prompt_index"
	WikiReadMcpGet       WikiReadSource = "mcp_get"
	WikiReadMcpSearch    WikiReadSource = "mcp_search"
)

const WikiSharedSlugPrefix = "shared:"

type WikiProposalKind string

const (
	WikiProposalCreate WikiProposalKind = "create"
	WikiProposalUpdate WikiProposalKind = "update"
	WikiProposalMove   WikiProposalKind = "move"
	WikiProposalDelete WikiProposalKind = "delete"
)

func IsValidWikiProposalKind(kind WikiProposalKind) bool {
	switch kind {
	case WikiProposalCreate, WikiProposalUpdate, WikiProposalMove, WikiProposalDelete:
		return true
	default:
		return false
	}
}

type WikiProposalDecision string

const (
	WikiProposalOpen           WikiProposalDecision = "open"
	WikiProposalApproved       WikiProposalDecision = "approved"
	WikiProposalNeedsResolving WikiProposalDecision = "needs_resolving"
	WikiProposalAccepted       WikiProposalDecision = "accepted"
	WikiProposalRejected       WikiProposalDecision = "rejected"
)

var (
	WikiProposalRevisable  = []WikiProposalDecision{WikiProposalOpen, WikiProposalApproved}
	WikiProposalUndecided  = []WikiProposalDecision{WikiProposalOpen, WikiProposalApproved, WikiProposalNeedsResolving}
	WikiProposalPublishing = []WikiProposalDecision{WikiProposalOpen, WikiProposalNeedsResolving}
)

type WikiProposalState string

const (
	WikiProposalStateOpen           WikiProposalState = "open"
	WikiProposalReady               WikiProposalState = "ready"
	WikiProposalStateApproved       WikiProposalState = "approved"
	WikiProposalStateNeedsResolving WikiProposalState = "needs_resolving"
	WikiProposalDiscarded           WikiProposalState = "discarded"
	WikiProposalStateAccepted       WikiProposalState = "accepted"
	WikiProposalStateRejected       WikiProposalState = "rejected"
)

func WikiProposalStateOf(decision WikiProposalDecision, runPhase string) WikiProposalState {
	switch {
	case decision == WikiProposalAccepted:
		return WikiProposalStateAccepted
	case decision == WikiProposalRejected:
		return WikiProposalStateRejected
	case decision == WikiProposalNeedsResolving:
		return WikiProposalStateNeedsResolving
	case runPhase == PhaseFailed || runPhase == PhaseCancelled:
		return WikiProposalDiscarded
	case decision == WikiProposalApproved:
		return WikiProposalStateApproved
	case runPhase == PhaseDone:
		return WikiProposalReady
	default:
		return WikiProposalStateOpen
	}
}
