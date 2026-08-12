import { describe, expect, it } from 'vitest';
import { DAY_MS, DateUtil, msUntilNextUtcDay } from './date.util';

describe('DateUtil', () => {
    describe('truncateTimeUtc', () => {
        it('drops the clock, keeping the UTC day', () => {
            expect(DateUtil.truncateTimeUtc('2026-08-10T23:59:59.999Z').toISOString()).toBe(
                '2026-08-10T00:00:00.000Z'
            );
        });

        it('reads the UTC day, not the local one', () => {
            expect(DateUtil.truncateTimeUtc('2026-08-09T22:00:00.000Z').toISOString()).toBe(
                '2026-08-09T00:00:00.000Z'
            );
        });
    });

    describe('toUtcDay', () => {
        it('stores the picked local day as UTC midnight of that day', () => {
            expect(DateUtil.toUtcDay(new Date(2026, 7, 10, 0, 0, 0)).toISOString()).toBe(
                '2026-08-10T00:00:00.000Z'
            );
        });

        it('keeps the day when the local clock is late in the evening', () => {
            expect(DateUtil.toUtcDay(new Date(2026, 7, 10, 23, 59, 59)).toISOString()).toBe(
                '2026-08-10T00:00:00.000Z'
            );
        });
    });

    describe('toLocalDay', () => {
        it('yields local midnight of the stored UTC day', () => {
            const local = DateUtil.toLocalDay('2026-08-10T00:00:00.000Z');
            expect([local.getFullYear(), local.getMonth(), local.getDate()]).toEqual([2026, 7, 10]);
            expect(local.getHours()).toBe(0);
        });

        it('reads a legacy instant by its UTC day', () => {
            expect(DateUtil.toLocalDay('2026-08-09T22:00:00.000Z').getDate()).toBe(9);
        });
    });

    describe('msUntilNextUtcDay', () => {
        it('waits only the remainder of the current UTC day', () => {
            expect(msUntilNextUtcDay(new Date('2026-08-10T23:30:00.000Z'))).toBe(30 * 60_000);
        });

        it('waits a full day at exactly UTC midnight', () => {
            expect(msUntilNextUtcDay(new Date('2026-08-10T00:00:00.000Z'))).toBe(DAY_MS);
        });

        // The day maths everywhere else buckets in UTC, so the tick must not follow
        // the local clock: for a UTC+2 user local midnight is two hours early.
        it('lands on the UTC boundary, not the local one', () => {
            const wait = msUntilNextUtcDay(new Date('2026-08-10T22:30:00.000Z'));
            expect(wait).toBe(90 * 60_000);
        });
    });

    it('round-trips a picked day through storage unchanged', () => {
        const picked = new Date(2026, 7, 10, 0, 0, 0);
        const stored = DateUtil.toUtcDay(picked);
        const seeded = DateUtil.toLocalDay(stored);
        expect(DateUtil.toUtcDay(seeded).toISOString()).toBe(stored.toISOString());
        expect(seeded.getDate()).toBe(10);
    });
});
