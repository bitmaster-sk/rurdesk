import { Track } from 'src/app/shared/tracker/model/track.model';
import { describe, expect, it } from 'vitest';
import { TrackedTimeChartSelection } from './tracked-time-chart-selection';

describe('TrackedTimeChartSelection', () => {
    const toDayKey = (d: Date): string => d.toISOString().slice(0, 10);

    const track = (over: Partial<Track>): Track => ({
        idTrack: 1,
        idUser: 1,
        idIssue: 11,
        idIssuePublic: 204,
        idProject: 1,
        issueTitle: 'Trackovanie',
        tracked: 3600,
        startAt: new Date('2026-10-21T09:00:00Z'),
        endAt: new Date('2026-10-21T10:00:00Z'),
        note: null,
        ...over
    });

    it('returns only the tracks of the issue and day of the clicked segment', () => {
        const tracks = [
            track({ idIssue: 11, startAt: new Date('2026-10-21T09:00:00Z') }),
            track({
                idTrack: 2,
                idIssue: 12,
                idIssuePublic: 205,
                startAt: new Date('2026-10-21T09:30:00Z')
            }),
            track({ idTrack: 3, startAt: new Date('2026-10-22T09:00:00Z') }),
            track({ idTrack: 4, startAt: new Date('2026-10-20T09:00:00Z') }),
            track({ idTrack: 5, startAt: null, endAt: new Date('2026-10-21T09:00:00Z') })
        ];

        const selected = TrackedTimeChartSelection.forSegment(tracks, 11, '2026-10-21', toDayKey);

        expect(selected.map(t => t.idTrack)).toEqual([1]);
    });

    it('orders the segment tracks by startAt', () => {
        const tracks = [
            track({ idTrack: 1, startAt: new Date('2026-10-21T15:00:00Z') }),
            track({ idTrack: 2, startAt: new Date('2026-10-21T08:00:00Z') }),
            track({ idTrack: 3, startAt: new Date('2026-10-21T11:00:00Z') })
        ];

        const selected = TrackedTimeChartSelection.forSegment(tracks, 11, '2026-10-21', toDayKey);

        expect(selected.map(t => t.idTrack)).toEqual([2, 3, 1]);
    });

    it('does not mutate the given tracks', () => {
        const tracks = [
            track({ idTrack: 2, startAt: new Date('2026-10-21T15:00:00Z') }),
            track({ idTrack: 1, startAt: new Date('2026-10-21T08:00:00Z') })
        ];
        const order = tracks.map(t => t.idTrack);

        TrackedTimeChartSelection.forSegment(tracks, 11, '2026-10-21', toDayKey);

        expect(tracks.map(t => t.idTrack)).toEqual(order);
    });
});
