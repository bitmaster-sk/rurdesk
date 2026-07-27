import { DurationFormatter } from './duration.formatter';

describe('DurationFormatter.durationToString', () => {
    it('formats h/m/s with compact suffixes', () => {
        expect(DurationFormatter.durationToString({ hours: 2, minutes: 30, seconds: 15 })).toBe(
            '2h 30m 15s'
        );
    });

    it('omits zero/absent units', () => {
        expect(DurationFormatter.durationToString({ hours: 1 })).toBe('1h');
    });

    it('returns an empty string for an empty duration', () => {
        expect(DurationFormatter.durationToString({})).toBe('');
    });
});
