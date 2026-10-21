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

describe('DurationFormatter.secondsToClock', () => {
    it('pads every part to two digits', () => {
        expect(DurationFormatter.secondsToClock(0)).toBe('00:00:00');
        expect(DurationFormatter.secondsToClock(65)).toBe('00:01:05');
    });

    it('keeps counting hours past a full day', () => {
        expect(DurationFormatter.secondsToClock(26 * 3600 + 61)).toBe('26:01:01');
    });

    it('clamps negative input to zero', () => {
        expect(DurationFormatter.secondsToClock(-5)).toBe('00:00:00');
    });
});

describe('DurationFormatter.millisecondsToString', () => {
    it('shows seconds under a minute and minutes with seconds above', () => {
        expect(DurationFormatter.millisecondsToString(4200)).toBe('4s');
        expect(DurationFormatter.millisecondsToString(99_000)).toBe('1m 39s');
    });
});
