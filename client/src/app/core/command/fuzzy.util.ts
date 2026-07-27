export interface HighlightSegment {
    text: string;
    hit: boolean;
}

export function fuzzyMatch(query: string, text: string): { matched: boolean; score: number } {
    if (!query) return { matched: true, score: 0 };
    const q = query.toLowerCase();
    const t = text.toLowerCase();
    let qi = 0,
        score = 0,
        run = 0,
        firstHit = -1;
    for (let i = 0; i < t.length && qi < q.length; i++) {
        if (t[i] === q[qi]) {
            if (firstHit < 0) firstHit = i;
            run += 1;
            score += run * 2;
            qi += 1;
        } else {
            run = 0;
        }
    }
    if (qi < q.length) return { matched: false, score: 0 };
    score += Math.max(0, 30 - firstHit);
    if (t.includes(q)) score += 40;
    return { matched: true, score };
}

export function highlight(text: string, query: string): HighlightSegment[] {
    if (!query) return [{ text, hit: false }];
    const q = query.toLowerCase();
    const segments: HighlightSegment[] = [];
    let qi = 0,
        buffer = '',
        bufferHit = false;
    const flush = (): void => {
        if (buffer) segments.push({ text: buffer, hit: bufferHit });
        buffer = '';
    };
    for (const ch of text) {
        const isHit = qi < q.length && ch.toLowerCase() === q[qi];
        if (isHit) qi += 1;
        if (buffer && isHit !== bufferHit) flush();
        bufferHit = isHit;
        buffer += ch;
    }
    flush();
    return segments;
}
