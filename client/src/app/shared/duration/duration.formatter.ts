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
}
