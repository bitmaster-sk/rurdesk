import { Duration } from 'date-fns';

export class DurationParser {
    public static stringToDuration(value: string): Duration {
        const duration: Duration = {};

        if (!value) {
            return duration;
        }

        const n = Number(value);
        if (!isNaN(n)) {
            return { hours: n };
        }

        const minMatch = value.match(/\d+(?=m)/);
        const secMatch = value.match(/\d+(?=s)/);
        const hoursMatch = value.match(/\d+(?=h)/);

        const minutes = minMatch?.length > 0 ? Number(minMatch[0]) : 0;
        const seconds = secMatch?.length > 0 ? Number(secMatch[0]) : 0;
        const hours = hoursMatch?.length > 0 ? Number(hoursMatch[0]) : 0;

        return { minutes, seconds, hours };
    }
}
