import { defer, Observable, timer } from 'rxjs';
import { map, repeat } from 'rxjs/operators';

export const DAY_MS = 86_400_000;

export const utcDayRollover$: Observable<Date> = defer(() =>
    timer(DateUtil.msUntilNextUtcDay(new Date()))
).pipe(
    repeat(),
    map(() => new Date())
);

export abstract class DateUtil {
    public static msUntilNextUtcDay(now: Date): number {
        const next = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() + 1);
        return next - now.getTime();
    }

    public static truncateTimeUtc(value: Date | string): Date {
        const date = value instanceof Date ? value : new Date(value);
        return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
    }

    public static toUtcDay(picked: Date): Date {
        return new Date(Date.UTC(picked.getFullYear(), picked.getMonth(), picked.getDate()));
    }

    public static toLocalDay(value: Date | string): Date {
        const date = value instanceof Date ? value : new Date(value);
        return new Date(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate());
    }
}
