// Minimal view-model the shared staged-issue card edits. Both the AI backlog
// (ProjectBuilderIssue) and the AI split (ProposedIssue) map to/from this shape.
export interface StagedIssue {
    ref: string;
    title: string;
    description: string;
    estimatedMinutes: number;
    idState: number | null;
    idSeverity: number | null;
}
