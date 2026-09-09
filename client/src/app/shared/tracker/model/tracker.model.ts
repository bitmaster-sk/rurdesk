import { Duration } from 'date-fns';

export interface Tracker {
    idTracker: number;
    idUser: number;
    idIssue: number;
    startAt: Date;
    pausedAt: Date | null;
    pausedSeconds: number;
    duration: Duration;
    idProject: number;
    idIssuePublic: number;
    issueTitle: string;
    projectName: string;
}
