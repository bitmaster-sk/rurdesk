import enLocale from '@fullcalendar/core/locales/en-gb';
import skLocale from '@fullcalendar/core/locales/sk';
import deLocale from '@fullcalendar/core/locales/de';

export const FULLCALENDAR_LOCALES = [enLocale, skLocale, deLocale];

const LOCALE_MAP: Record<string, typeof enLocale> = {
    en: enLocale,
    sk: skLocale,
    de: deLocale
};

/**
 * Resolve the active app language to a FullCalendar locale object.
 * Unknown or missing codes fall back to English.
 */
export function resolveFullCalendarLocale(lang?: string): typeof enLocale {
    if (!lang) {
        return enLocale;
    }

    const locale = LOCALE_MAP[lang];
    if (locale) {
        return locale;
    }

    return enLocale;
}
