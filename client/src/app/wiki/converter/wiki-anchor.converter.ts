import { WikiSlugConverter } from './wiki-slug.converter';

export abstract class WikiAnchorConverter {
    public static readonly idPrefix = 'wiki-h-';

    public static toAnchors(headings: string[]): string[] {
        const seen = new Map<string, number>();
        return headings.map(heading => {
            const slug = WikiSlugConverter.toSlug(heading) || 'section';
            const count = seen.get(slug) ?? 0;
            seen.set(slug, count + 1);
            return count === 0 ? slug : `${slug}-${count}`;
        });
    }

    public static toHeadings(markdown: string): string[] {
        const headings: string[] = [];
        let fence: string | null = null;
        for (const line of markdown.split('\n')) {
            const fenceMatch = /^ {0,3}(`{3,}|~{3,})/.exec(line);
            if (fenceMatch) {
                fence =
                    fence === null
                        ? fenceMatch[1]
                        : line.trimStart().startsWith(fence)
                          ? null
                          : fence;
                continue;
            }
            const heading = fence === null ? /^ {0,3}#{1,6}\s+(.+?)\s*#*\s*$/.exec(line) : null;
            if (heading) {
                headings.push(heading[1]);
            }
        }
        return headings;
    }

    public static toElementId(anchor: string): string {
        return `${WikiAnchorConverter.idPrefix}${anchor}`;
    }
}
