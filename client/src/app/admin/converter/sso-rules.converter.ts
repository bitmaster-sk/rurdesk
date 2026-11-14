export abstract class SsoRulesConverter {
    public static toDomains(text: string): string[] {
        return text.split(/[\s,]+/).filter(domain => domain.length > 0);
    }

    public static toText(domains: readonly string[]): string {
        return domains.join('\n');
    }
}
