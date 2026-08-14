export interface SprintStats {
    totalPoints: number;
    donePoints: number;
    startPoints: number;
    progressPoints: number;
    totalIssues: number;
    doneIssues: number;
    startIssues: number;
    progressIssues: number;
    pointedIssues: number;
    rolledOverIssues?: number;
    frozenTotalPoints?: number;
    frozenDonePoints?: number;
    frozenTotalIssues?: number;
    frozenDoneIssues?: number;
    frozenPointedIssues?: number;
}
