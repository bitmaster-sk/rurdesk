import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

export abstract class TrackedTimeChartText {
    public static fit(text: string, maxWidth: number, measure: (t: string) => number): string {
        if (measure(text) <= maxWidth) {
            return text;
        }
        for (let length = text.length - 1; length >= 2; length--) {
            const candidate = `${text.slice(0, length)}…`;
            if (measure(candidate) <= maxWidth) {
                return candidate;
            }
        }
        return '';
    }

    public static segment(
        width: number,
        height: number,
        num: string,
        title: string,
        measure: (t: string) => number
    ): string | null {
        if (height < 14) {
            return null;
        }
        const inner = width - 12;
        if (measure(num) > inner) {
            return null;
        }
        const fitted = TrackedTimeChartText.fit(
            title,
            inner - measure(num) - measure(' '),
            measure
        );
        return fitted ? `${num} ${fitted}` : num;
    }

    public static formatHours(hours: number): string {
        return DurationFormatter.durationToString(
            DurationConverter.secondsToDuration(Math.round(hours * 60) * 60)
        );
    }
}
