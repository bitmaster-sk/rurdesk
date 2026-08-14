import { SprintState } from '../constants/sprint-state.enum';

export interface SprintBurndownDay {
    day: string;
    totalPoints: number | null;
    donePoints: number | null;
    remainingPoints: number | null;
    totalIssues: number | null;
    doneIssues: number | null;
    remainingIssues: number | null;
    snapshot: boolean;
}

export interface SprintBurndown {
    idSprint: number;
    startAt: string;
    endAt: string;
    state: SprintState;
    days: SprintBurndownDay[];
}
