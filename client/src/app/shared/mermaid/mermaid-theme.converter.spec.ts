import { MermaidThemeConverter } from './mermaid-theme.converter';

describe('MermaidThemeConverter.toThemeVariables', () => {
    const tokens: Record<string, string> = {
        '--ui-color-primary-weak': '#eef2ff',
        '--ui-color-primary': '#3b5bdb',
        '--ui-color-text': '#1a3263',
        '--ui-color-text-muted': '#64748b',
        '--ui-color-canvas': '#f8fafc',
        '--ui-color-surface': '#ffffff',
        '--ui-color-border': '#e2e8f0',
        '--ui-color-danger': '#dc2626'
    };
    const read = (token: string): string => tokens[token] ?? '';

    it('maps every design token to its mermaid theme variable', () => {
        const vars = MermaidThemeConverter.toThemeVariables(read, 'Lato, sans-serif');
        expect(vars).toEqual({
            primaryColor: '#eef2ff',
            primaryBorderColor: '#3b5bdb',
            primaryTextColor: '#1a3263',
            textColor: '#1a3263',
            lineColor: '#64748b',
            secondaryColor: '#f8fafc',
            tertiaryColor: '#f8fafc',
            background: '#ffffff',
            mainBkg: '#ffffff',
            clusterBorder: '#e2e8f0',
            nodeBorder: '#e2e8f0',
            errorBkgColor: '#dc2626',
            fontFamily: 'Lato, sans-serif'
        });
    });

    it('reads every value through the caller-supplied reader, not the DOM', () => {
        const vars = MermaidThemeConverter.toThemeVariables(() => '#012345', 'Mono');
        expect(new Set(Object.values(vars))).toEqual(new Set(['#012345', 'Mono']));
    });
});
