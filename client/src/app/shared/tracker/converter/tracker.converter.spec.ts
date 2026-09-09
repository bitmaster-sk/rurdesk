import { describe, expect, it } from 'vitest';
import { Tracker } from '../model/tracker.model';
import { TrackerConverter } from './tracker.converter';

const tracker = (over: Partial<Tracker>): Tracker => {
    const base: Tracker = {
        idTracker: 1,
        idUser: 1,
        idIssue: 10,
        startAt: new Date('2026-09-08T10:00:00Z'),
        pausedAt: null,
        pausedSeconds: 0,
        duration: {},
        idProject: 2,
        idIssuePublic: 142,
        issueTitle: 'Global time tracker',
        projectName: 'RuRdesk'
    };
    return { ...base, ...over };
};

describe('TrackerConverter.toElapsedSeconds', () => {
    const now = new Date('2026-09-08T12:00:00Z');

    it('counts the time since the timer started', () => {
        expect(TrackerConverter.toElapsedSeconds(tracker({}), now)).toBe(7200);
    });

    it('subtracts time already spent paused', () => {
        expect(TrackerConverter.toElapsedSeconds(tracker({ pausedSeconds: 1800 }), now)).toBe(5400);
    });

    it('stops counting while the timer is paused', () => {
        const paused = tracker({ pausedAt: new Date('2026-09-08T11:00:00Z') });
        expect(TrackerConverter.toElapsedSeconds(paused, now)).toBe(3600);
    });

    it('never reports a negative duration', () => {
        const skewed = tracker({ startAt: new Date('2026-09-08T13:00:00Z') });
        expect(TrackerConverter.toElapsedSeconds(skewed, now)).toBe(0);
    });
});

describe('TrackerConverter.toTracker', () => {
    it('revives the dates the API sends as strings', () => {
        const revived = TrackerConverter.toTracker({
            startAt: '2026-09-08T10:00:00Z',
            pausedAt: '2026-09-08T11:00:00Z'
        } as unknown as Tracker);

        expect(revived.startAt).toBeInstanceOf(Date);
        expect(revived.pausedAt).toBeInstanceOf(Date);
        expect(revived.pausedSeconds).toBe(0);
    });

    it('leaves a running timer without a pause timestamp', () => {
        const revived = TrackerConverter.toTracker({
            startAt: '2026-09-08T10:00:00Z'
        } as unknown as Tracker);

        expect(revived.pausedAt).toBeNull();
    });
});
