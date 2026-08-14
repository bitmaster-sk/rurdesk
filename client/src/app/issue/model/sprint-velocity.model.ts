export interface SprintVelocity {
    idSprint: number;
    name: string;
    endAt: string;
    donePoints: number;
    doneIssues: number;
    plannedPoints?: number;
    plannedIssues?: number;
    frozen: boolean;
}

export interface SprintVelocityAverages {
    points: number;
    issues: number;
}
