import { Duration } from 'date-fns';

export interface Tracker {
    idTracker: number;
    idUser: number;
    idIssue: number;
    startAt: Date;
    duration: Duration;
    idProject: number;
    idIssuePublic: number;
}
