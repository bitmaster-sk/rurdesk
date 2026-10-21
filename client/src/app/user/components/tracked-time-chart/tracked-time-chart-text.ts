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

    public static hours(hours: number): string {
        const h = Math.floor(hours);
        const m = Math.round((hours - h) * 60);
        return m === 0 ? `${h}h` : `${h}h ${m}m`;
    }
}
