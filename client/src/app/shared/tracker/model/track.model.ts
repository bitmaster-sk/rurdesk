export interface Track {
    idTrack: number;
    idUser: number;
    idIssue: number;
    idIssuePublic: number;
    idProject: number;
    issueTitle: string;
    tracked: number | null;
    startAt: Date | null;
    endAt: Date | null;
}

export interface CreateTrackReq {
    idIssue: number;
    tracked?: number | null;
    startAt?: Date | null;
    endAt?: Date | null;
}

export interface TrackUpdate extends CreateTrackReq {
    idTrack: number;
}

export interface TrackForm {
    idTrack: number | null;
    idUser: number;
    idIssue: number;
    tracked: number | null;
    endAt: Date | null;
}
