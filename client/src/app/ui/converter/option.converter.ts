export abstract class OptionConverter {
    /**
     * Render an option (or one of its properties) as label text. Only primitives have a
     * meaningful text form here — anything else renders empty rather than `[object Object]`.
     */
    public static toLabel(value: unknown): string {
        if (typeof value === 'string') {
            return value;
        }
        if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
            return String(value);
        }
        return '';
    }
}
