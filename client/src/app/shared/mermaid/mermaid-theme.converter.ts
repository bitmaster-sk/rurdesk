/**
 * Maps the app's `--ui-*` design tokens to mermaid `themeVariables`. Pure: the
 * caller passes a token reader (so the unit spec needs no DOM) and the body
 * font family. Light theme only — the app has no dark mode.
 */
export abstract class MermaidThemeConverter {
    public static toThemeVariables(
        read: (token: string) => string,
        fontFamily: string
    ): Record<string, string> {
        const text = read('--ui-color-text');
        const canvas = read('--ui-color-canvas');
        const surface = read('--ui-color-surface');
        const border = read('--ui-color-border');
        return {
            primaryColor: read('--ui-color-primary-weak'),
            primaryBorderColor: read('--ui-color-primary'),
            primaryTextColor: text,
            textColor: text,
            lineColor: read('--ui-color-text-muted'),
            secondaryColor: canvas,
            tertiaryColor: canvas,
            background: surface,
            mainBkg: surface,
            clusterBorder: border,
            nodeBorder: border,
            errorBkgColor: read('--ui-color-danger'),
            fontFamily
        };
    }
}
