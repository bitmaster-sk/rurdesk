/**
 * Classifies a row in `messages.message`. Mirrors the Postgres
 * `messages.message_kind` enum and the Go `constants.MessageKind` type.
 */
export enum MessageKind {
    Comment = 'comment',
    BrainstormingQuestion = 'brainstorming_question',
    BrainstormingComplete = 'brainstorming_complete',
    Design = 'design',
    ImplementationPlan = 'implementation_plan',
    PullRequestPushed = 'pull_request_pushed',
    ImplementationDone = 'implementation_done',
    ReviewReply = 'review_reply'
}
