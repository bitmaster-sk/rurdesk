import { describe, expect, it } from 'vitest';
import { UI_DATETIME_PATTERN, UI_DATE_PATTERN, uiFormatDate, uiParseDate } from './ui-date-format';

describe('ui-date-format', () => {
    it('formats a date with the localized date pattern (comma, no time)', () => {
        const d = new Date(2026, 6, 3); // Jul 3, 2026 (month is 0-based)
        expect(uiFormatDate(d, UI_DATE_PATTERN)).toBe('Jul 3, 2026');
    });

    it('formats a datetime with the combined pattern (comma before time, 12h)', () => {
        const d = new Date(2026, 6, 3, 14, 30);
        expect(uiFormatDate(d, UI_DATETIME_PATTERN)).toBe('Jul 3, 2026, 2:30 PM');
    });

    it('round-trips a date through format → parse', () => {
        const d = new Date(2026, 6, 3);
        const parsed = uiParseDate(uiFormatDate(d, UI_DATE_PATTERN), UI_DATE_PATTERN);
        expect(parsed.getFullYear()).toBe(2026);
        expect(parsed.getMonth()).toBe(6);
        expect(parsed.getDate()).toBe(3);
    });

    it('round-trips a datetime through format → parse (incl. 12h time)', () => {
        const d = new Date(2026, 6, 3, 14, 30);
        const parsed = uiParseDate(uiFormatDate(d, UI_DATETIME_PATTERN), UI_DATETIME_PATTERN);
        expect(parsed.getHours()).toBe(14);
        expect(parsed.getMinutes()).toBe(30);
    });

    it('parse yields an Invalid Date for garbage input (does not throw)', () => {
        const parsed = uiParseDate('not a date', UI_DATE_PATTERN);
        expect(Number.isNaN(parsed.getTime())).toBe(true);
    });
});
