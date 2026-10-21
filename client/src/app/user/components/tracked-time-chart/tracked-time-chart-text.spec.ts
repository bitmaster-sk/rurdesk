import { TrackedTimeChartText } from './tracked-time-chart-text';

describe('TrackedTimeChartText.fit', () => {
    const measure = (t: string): number => t.length * 6;

    it('returns a text that fits unchanged', () => {
        expect(TrackedTimeChartText.fit('Trackovanie', 72, measure)).toBe('Trackovanie');
    });

    it('shortens a long text to end with an ellipsis', () => {
        expect(TrackedTimeChartText.fit('Trackovanie času', 72, measure)).toBe('Trackovanie…');
    });

    it('returns an empty string when too little space remains', () => {
        expect(TrackedTimeChartText.fit('Trackovanie času', 12, measure)).toBe('');
    });
});

describe('TrackedTimeChartText.segment', () => {
    const measure = (t: string): number => t.length * 6;
    const title = 'Trackovanie času';

    it('returns null for a low segment', () => {
        expect(TrackedTimeChartText.segment(114, 10, '#204', title, measure)).toBeNull();
    });

    it('returns null when the number does not fit', () => {
        expect(TrackedTimeChartText.segment(30, 26, '#204', title, measure)).toBeNull();
    });

    it('returns only the number when the title does not fit', () => {
        expect(TrackedTimeChartText.segment(50, 26, '#204', title, measure)).toBe('#204');
    });

    it('returns the number with a fitted title for a wide segment', () => {
        expect(TrackedTimeChartText.segment(114, 26, '#204', title, measure)).toBe(
            '#204 Trackovanie…'
        );
    });
});

describe('TrackedTimeChartText.hours', () => {
    it('formats a whole hour without minutes', () => {
        expect(TrackedTimeChartText.hours(7)).toBe('7h');
    });

    it('formats a fractional hour with minutes', () => {
        expect(TrackedTimeChartText.hours(6.5)).toBe('6h 30m');
        expect(TrackedTimeChartText.hours(0.25)).toBe('0h 15m');
    });
});
