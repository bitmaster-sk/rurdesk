export interface WikiSnippetPart {
    text: string;
    isMatch: boolean;
}

export abstract class WikiSnippetConverter {
    public static toParts(snippet: string): WikiSnippetPart[] {
        const parts: WikiSnippetPart[] = [];
        const pattern = /<<(.*?)>>/g;
        let last = 0;
        for (const match of snippet.matchAll(pattern)) {
            const index = match.index ?? 0;
            if (index > last) {
                parts.push({ text: snippet.slice(last, index), isMatch: false });
            }
            parts.push({ text: match[1], isMatch: true });
            last = index + match[0].length;
        }
        if (last < snippet.length) {
            parts.push({ text: snippet.slice(last), isMatch: false });
        }
        return parts;
    }
}
