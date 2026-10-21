package constants

// MessageKind classifies a row in messages.message. Persisted as the
// messages.message_kind enum.
type MessageKind string

const (
	MessageKindComment               MessageKind = "comment"
	MessageKindBrainstormingQuestion MessageKind = "brainstorming_question"
	MessageKindBrainstormingComplete MessageKind = "brainstorming_complete"
	MessageKindDesign                MessageKind = "design"
	MessageKindImplementationPlan    MessageKind = "implementation_plan"
	MessageKindPullRequestPushed     MessageKind = "pull_request_pushed"
	MessageKindImplementationDone    MessageKind = "implementation_done"
	MessageKindReviewReply           MessageKind = "review_reply"
)

var AgentMessageKinds = []string{
	string(MessageKindBrainstormingQuestion),
	string(MessageKindBrainstormingComplete),
	string(MessageKindDesign),
	string(MessageKindImplementationPlan),
	string(MessageKindPullRequestPushed),
	string(MessageKindImplementationDone),
	string(MessageKindReviewReply),
}
