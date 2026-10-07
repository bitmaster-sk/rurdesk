import { WikiCalloutKind } from '../constants/wiki-callout-kind.enum';

export abstract class WikiCalloutConverter {
    public static readonly labelKeys: Record<WikiCalloutKind, string> = {
        [WikiCalloutKind.Note]: 'WIKI.CALLOUT.NOTE',
        [WikiCalloutKind.Tip]: 'WIKI.CALLOUT.TIP',
        [WikiCalloutKind.Important]: 'WIKI.CALLOUT.IMPORTANT',
        [WikiCalloutKind.Warning]: 'WIKI.CALLOUT.WARNING',
        [WikiCalloutKind.Caution]: 'WIKI.CALLOUT.CAUTION'
    };

    private static readonly fencePattern = /^ {0,3}(`{3,}|~{3,})/;
    private static readonly openPattern =
        /^ {0,3}> ?\[!(note|tip|important|warning|caution)\]\s*$/i;
    private static readonly quotePattern = /^ {0,3}> ?/;

    public static toLabels(translate: (key: string) => string): Record<WikiCalloutKind, string> {
        const labels = { ...WikiCalloutConverter.labelKeys };
        for (const kind of Object.values(WikiCalloutKind)) {
            labels[kind] = translate(WikiCalloutConverter.labelKeys[kind]);
        }
        return labels;
    }

    public static toHtml(body: string, labels: Record<WikiCalloutKind, string>): string {
        const lines = body.split('\n');
        const result: string[] = [];
        let fence: string | null = null;
        for (let index = 0; index < lines.length; index++) {
            const line = lines[index];
            if (fence) {
                result.push(line);
                if (line.trimStart().startsWith(fence)) {
                    fence = null;
                }
                continue;
            }
            const fenceMatch = WikiCalloutConverter.fencePattern.exec(line);
            if (fenceMatch) {
                fence = fenceMatch[1];
                result.push(line);
                continue;
            }
            const openMatch = WikiCalloutConverter.openPattern.exec(line);
            if (!openMatch) {
                result.push(line);
                continue;
            }
            const kind = openMatch[1].toLowerCase() as WikiCalloutKind;
            const content: string[] = [];
            while (
                index + 1 < lines.length &&
                WikiCalloutConverter.quotePattern.test(lines[index + 1])
            ) {
                index++;
                content.push(lines[index].replace(WikiCalloutConverter.quotePattern, ''));
            }
            result.push(
                `<div class="wiki-callout wiki-callout--${kind}">`,
                `<div class="wiki-callout__title">${WikiCalloutConverter.escape(labels[kind])}</div>`,
                '',
                ...content,
                '',
                '</div>'
            );
            if (index + 1 < lines.length && lines[index + 1].trim() !== '') {
                result.push('');
            }
        }
        return result.join('\n');
    }

    private static escape(text: string): string {
        return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
}
