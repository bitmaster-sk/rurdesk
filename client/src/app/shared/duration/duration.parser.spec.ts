import { DurationParser } from './duration.parser';

describe('DurationParser.stringToDuration', () => {
    it('returns an empty duration for empty input', () => {
        expect(DurationParser.stringToDuration('')).toEqual({});
    });

    it('treats a bare number as hours', () => {
        expect(DurationParser.stringToDuration('2')).toEqual({ hours: 2 });
        expect(DurationParser.stringToDuration('90')).toEqual({ hours: 90 });
    });

    it('parses combined h/m/s tokens', () => {
        expect(DurationParser.stringToDuration('1h2m3s')).toEqual({
            hours: 1,
            minutes: 2,
            seconds: 3
        });
    });

    it('parses partial tokens, defaulting the rest to 0', () => {
        expect(DurationParser.stringToDuration('45m')).toEqual({
            hours: 0,
            minutes: 45,
            seconds: 0
        });
        expect(DurationParser.stringToDuration('30s')).toEqual({
            hours: 0,
            minutes: 0,
            seconds: 30
        });
    });

    it('returns zeros for an unparseable string', () => {
        expect(DurationParser.stringToDuration('abc')).toEqual({
            hours: 0,
            minutes: 0,
            seconds: 0
        });
    });
});
