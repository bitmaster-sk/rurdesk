import { Track } from 'src/app/shared/tracker/model/track.model';

export abstract class TrackedTimeChartSelection {
    public static forSegment(
        tracks: Track[],
        idIssue: number,
        dayKey: string,
        toDayKey: (d: Date) => string
    ): Track[] {
        return tracks
            .filter(
                t => t.idIssue === idIssue && t.startAt !== null && toDayKey(t.startAt) === dayKey
            )
            .sort((a, b) => (a.startAt?.getTime() ?? 0) - (b.startAt?.getTime() ?? 0));
    }
}
