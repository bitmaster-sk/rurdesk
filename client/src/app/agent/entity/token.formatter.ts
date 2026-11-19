export abstract class TokenFormatter {
    /** A token count in short form: `999`, `1.2k`, `2.3M`. */
    public static format(tokens: number): string {
        if (tokens < 1000) {
            return `${tokens}`;
        }
        if (tokens < 1_000_000) {
            return `${Math.round(tokens / 100) / 10}k`;
        }
        return `${Math.round(tokens / 100_000) / 10}M`;
    }
}
