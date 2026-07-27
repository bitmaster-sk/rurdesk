export interface Track {
    idTrack?: number;
    idUser?: number;
    idIssue: number;
    idIssuePublic?: number;
    idProject?: number;
    issueTitle?: string;
    tracked?: number;
    startAt?: Date;
    endAt?: Date;
}
