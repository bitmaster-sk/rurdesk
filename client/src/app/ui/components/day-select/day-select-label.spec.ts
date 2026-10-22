import { describe, expect, it } from 'vitest';
import { UiDaySelectLabel, UiDaySelectRelative } from './day-select-label';

describe('UiDaySelectLabel', () => {
    it('labels the same calendar day as Today regardless of the time of day', () => {
        const today = new Date(2026, 9, 21, 15, 30);

        expect(UiDaySelectLabel.relative(new Date(2026, 9, 21), today)).toBe(
            UiDaySelectRelative.Today
        );
        expect(UiDaySelectLabel.relative(new Date(2026, 9, 21, 23, 59, 59), today)).toBe(
            UiDaySelectRelative.Today
        );
    });

    it('labels exactly one calendar day back as Yesterday', () => {
        const today = new Date(2026, 9, 21, 0, 0);

        expect(UiDaySelectLabel.relative(new Date(2026, 9, 20, 23, 59), today)).toBe(
            UiDaySelectRelative.Yesterday
        );
    });

    it('labels Yesterday across a month rollover: 1 Nov → 31 Oct', () => {
        const novFirst = new Date(2026, 10, 1, 9, 0);

        expect(UiDaySelectLabel.relative(new Date(2026, 9, 31, 22, 0), novFirst)).toBe(
            UiDaySelectRelative.Yesterday
        );
    });

    it('labels Yesterday across a year rollover: 1 Jan → 31 Dec', () => {
        const janFirst = new Date(2027, 0, 1);

        expect(UiDaySelectLabel.relative(new Date(2026, 11, 31, 23, 59), janFirst)).toBe(
            UiDaySelectRelative.Yesterday
        );
    });

    it('returns null for days older than yesterday', () => {
        const today = new Date(2026, 9, 21, 12, 0);

        expect(UiDaySelectLabel.relative(new Date(2026, 9, 19), today)).toBeNull();
        expect(UiDaySelectLabel.relative(new Date(2026, 9, 1), today)).toBeNull();
        expect(UiDaySelectLabel.relative(new Date(2025, 0, 1), today)).toBeNull();
    });

    it('returns null for future days', () => {
        const today = new Date(2026, 9, 21);

        expect(UiDaySelectLabel.relative(new Date(2026, 9, 22), today)).toBeNull();
        expect(UiDaySelectLabel.relative(new Date(2026, 10, 1), today)).toBeNull();
    });
});
