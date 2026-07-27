import {
    Duration,
    differenceInSeconds,
    differenceInHours,
    differenceInMinutes,
    sub
} from 'date-fns';
import { Tracker } from '../tracker/model/tracker.model';

export class DurationConverter {
    public static trackerToDuration(tracker: Tracker): Duration {
        const duration: Duration = {};

        if (!tracker?.startAt) {
            return {};
        }

        const now = new Date();
        duration.hours = Math.abs(differenceInHours(tracker.startAt, now));

        const remainingMinutes = sub(tracker.startAt, { hours: -duration.hours });
        duration.minutes = Math.abs(differenceInMinutes(remainingMinutes, now));

        const remainingSeconds = sub(remainingMinutes, { minutes: -duration.minutes });
        duration.seconds = Math.abs(differenceInSeconds(remainingSeconds, now));

        return duration;
    }

    public static secondsToDuration(seconds: number): Duration {
        const duration: Duration = {};

        const remainingAfteHours = seconds % (60 * 60);
        duration.hours = Math.floor(seconds / (60 * 60));

        const remainingAfterMinutes = remainingAfteHours % 60;
        duration.minutes = Math.floor(remainingAfteHours / 60);

        duration.seconds = remainingAfterMinutes;
        return duration;
    }

    public static durationToSeconds(duration: Duration): number {
        return (
            (duration?.seconds ? duration.seconds : 0) +
            (duration?.minutes ? duration.minutes * 60 : 0) +
            (duration?.hours ? duration.hours * 60 * 60 : 0) +
            (duration?.days ? duration.days * 60 * 60 * 24 : 0)
        );
    }
}
