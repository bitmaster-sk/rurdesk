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
        return tracker;
    }
}
