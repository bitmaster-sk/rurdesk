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

        const minutes = minMatch ? Number(minMatch[0]) : 0;
        const seconds = secMatch ? Number(secMatch[0]) : 0;
        const hours = hoursMatch ? Number(hoursMatch[0]) : 0;

        return { minutes, seconds, hours };
    }
}
