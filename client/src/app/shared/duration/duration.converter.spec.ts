import { DurationConverter } from './duration.converter';

describe('DurationConverter.secondsToDuration', () => {
    it('splits seconds into h/m/s', () => {
        expect(DurationConverter.secondsToDuration(3661)).toEqual({
            hours: 1,
            minutes: 1,
            seconds: 1
        });
    });

    it('returns all zeros for 0 seconds', () => {
        expect(DurationConverter.secondsToDuration(0)).toEqual({
            hours: 0,
            minutes: 0,
            seconds: 0
        });
    });
});

describe('DurationConverter.durationToSeconds', () => {
    it('sums seconds, minutes, hours and days', () => {
        expect(DurationConverter.durationToSeconds({ hours: 1, minutes: 1, seconds: 1 })).toBe(
            3661
        );
        expect(DurationConverter.durationToSeconds({ days: 1 })).toBe(86400);
    });

    it('returns 0 for an empty duration', () => {
        expect(DurationConverter.durationToSeconds({})).toBe(0);
    });
});

describe('DurationConverter round-trip', () => {
    it('seconds → duration → seconds is stable', () => {
        const seconds = 7325; // 2h 2m 5s
        const duration = DurationConverter.secondsToDuration(seconds);
        expect(DurationConverter.durationToSeconds(duration)).toBe(seconds);
    });
});
