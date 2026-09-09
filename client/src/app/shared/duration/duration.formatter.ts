import { Duration, formatDuration } from 'date-fns';

const ops = {
    xSeconds: '{{count}}s',
    xMinutes: '{{count}}m',
    xHours: '{{count}}h'
};

const locale = {
    locale: {
        formatDistance: (token: string, count: number): string =>
            (ops as Record<string, string>)[token]?.replace('{{count}}', `${count}`) ?? ''
    }
};

export class DurationFormatter {
    public static durationToString(duration: Duration): string {
        return formatDuration(duration, locale);
    }

    public static secondsToClock(seconds: number): string {
        const safe = Math.max(0, Math.floor(seconds));
        const parts = [Math.floor(safe / 3600), Math.floor((safe % 3600) / 60), safe % 60];
        return parts.map(part => `${part}`.padStart(2, '0')).join(':');
    }
}
