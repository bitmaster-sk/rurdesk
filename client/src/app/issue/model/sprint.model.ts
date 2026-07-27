export interface Sprint {
    idSprint: number;
    idProject: number;
    name: string;
    startAt: string;
    endAt: string;
    state: 'planned' | 'closed'; // no persisted 'active' — "current" is date-derived
}

export interface SprintStats {
    idSprint: number;
    totalPoints: number;
    donePoints: number;
    totalIssues: number;
    doneIssues: number;
}
