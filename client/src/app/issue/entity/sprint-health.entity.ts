import { SprintPhase } from '../constants/sprint-phase.enum';
import { SprintUnit } from '../constants/sprint-unit.enum';

export interface SprintHealthBase {
    unit: SprintUnit;
    isUnitForced: boolean;
    committed: number;
    done: number;
    inProgress: number;
    notStarted: number;
}

export interface SprintHealthBacklog extends SprintHealthBase {
    phase: SprintPhase.Backlog;
}

export interface SprintHealthPlanned extends SprintHealthBase {
    phase: SprintPhase.NotStarted;
    avgVelocity: number | null;
    isOverCommitted: boolean;
}

export interface SprintHealthForecast {
    isOnTrack: boolean;
    paceSoFar: number;
    paceNeeded: number;
    gap: number;
}

export interface SprintHealthRunning extends SprintHealthBase {
    phase: SprintPhase.Running;
    daysTotal: number;
    daysElapsed: number;
    daysLeft: number;
    isTooEarly: boolean;
    forecast: SprintHealthForecast | null;
}

export interface SprintHealthClosed extends SprintHealthBase {
    phase: SprintPhase.Closed;
    isFrozen: boolean;
    rolledOverIssues: number | null;
}

export interface SprintForecastFinish {
    finish: Date;
    daysLate: number;
}

export type SprintHealth =
    SprintHealthBacklog | SprintHealthPlanned | SprintHealthRunning | SprintHealthClosed;
