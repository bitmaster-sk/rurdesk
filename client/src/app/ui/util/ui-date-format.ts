import { format, parse } from 'date-fns';
import { enUS } from 'date-fns/locale';
import type { Locale } from 'date-fns';

/**
 * Single source of truth for datepicker display formatting. flatpickr's own
 * PHP-style token grammar never enters the codebase — the `[uiDatepicker]`
 * directive delegates flatpickr's `formatDate`/`parseDate` hooks to these
 * date-fns helpers, so format and parse always agree and stay locale-driven.
 *
 * Patterns are date-fns LOCALIZED tokens (locale decides day/month order and
 * 12h/24h), not fixed strings — currently en-US.
 */

/** Date only, e.g. `Jul 3, 2026`. */
export const UI_DATE_PATTERN = 'PP';
/** Date + time (combined localized token → comma), e.g. `Jul 3, 2026, 2:30 PM`. */
export const UI_DATETIME_PATTERN = 'PPp';

// App is single-language en today (app.module defaultLanguage 'en', no
// LOCALE_ID override). When i18n arrives, map the active language here.
const activeLocale: Locale = enUS;

export function uiFormatDate(date: Date, pattern: string): string {
    return format(date, pattern, { locale: activeLocale });
}

export function uiParseDate(str: string, pattern: string): Date {
    // date-fns parse requires a reference date (3rd arg) for fields the pattern
    // doesn't cover.
    return parse(str, pattern, new Date(), { locale: activeLocale });
}
