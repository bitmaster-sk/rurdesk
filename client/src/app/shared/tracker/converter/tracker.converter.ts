import { Track } from '../model/track.model';
import { Tracker } from '../model/tracker.model';

export abstract class TrackerConverter {
    public static toTracks(tracks: Track[]): Track[] {
        return tracks.map(track => TrackerConverter.toTrack(track));
    }

    public static toTrack(track: Track): Track {
        track.startAt = track.startAt ? new Date(track.startAt) : null;
        track.endAt = track.endAt ? new Date(track.endAt) : null;
        return track;
    }

    public static toTracker(tracker: Tracker): Tracker {
        tracker.startAt = new Date(tracker.startAt);
        tracker.pausedAt = tracker.pausedAt ? new Date(tracker.pausedAt) : null;
        tracker.pausedSeconds = tracker.pausedSeconds ?? 0;
        return tracker;
    }

    public static toElapsedSeconds(tracker: Tracker, now: Date = new Date()): number {
        if (!tracker?.startAt) {
            return 0;
        }
        const until = tracker.pausedAt ?? now;
        const elapsed =
            Math.floor((until.getTime() - tracker.startAt.getTime()) / 1000) -
            (tracker.pausedSeconds ?? 0);
        return elapsed > 0 ? elapsed : 0;
    }
}
