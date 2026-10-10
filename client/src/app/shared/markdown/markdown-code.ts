export abstract class MarkdownCode {
    public static readonly pattern = /(```[\s\S]*?```|`[^`\n]*`)/;

    public static outsideCode(text: string): string[] {
        return text.split(MarkdownCode.pattern).filter((_, index) => index % 2 === 0);
    }

    public static replaceOutsideCode(text: string, replace: (part: string) => string): string {
        return text
            .split(MarkdownCode.pattern)
            .map((part, index) => (index % 2 === 1 ? part : replace(part)))
            .join('');
    }
}
