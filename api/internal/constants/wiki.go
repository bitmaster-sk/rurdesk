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
