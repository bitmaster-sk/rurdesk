package constants

const (
	NotificationTypeComment              = "comment"
	NotificationTypeMention              = "mention"
	NotificationTypeAssigned             = "assigned"
	NotificationTypeStateChanged         = "state_changed"
	NotificationTypeSeverityEscalated    = "severity_escalated"
	NotificationTypeSeverityDeescalated  = "severity_deescalated"
	NotificationTypeTeamJoined           = "team_joined"
	NotificationTypeQualityScored        = "quality_scored"
	NotificationTypeWikiProposalReady    = "wiki_proposal_ready"
	NotificationTypeWikiProposalConflict = "wiki_proposal_conflict"

	NotificationSourceAgent = "agent"

	NotificationRefTypeIssue = "issue"
	NotificationRefTypeTeam  = "team"
)
