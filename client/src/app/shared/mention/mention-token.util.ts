export type MentionPart =
    { type: 'text'; text: string } | { type: 'mention'; idUser: number; name: string };

// Matches @[name](user:id). Name may contain spaces but not ']'. `g` for iteration.
export const MENTION_TOKEN_RE = /@\[([^\]]+)\]\(user:(\d+)\)/g;

export function serializeMention(idUser: number, name: string): string {
    return `@[${name}](user:${idUser})`;
}

export function parseMentionParts(body: string): MentionPart[] {
    const parts: MentionPart[] = [];
    const re = new RegExp(MENTION_TOKEN_RE.source, 'g');
    let lastIndex = 0;
    let m: RegExpExecArray | null;
    while ((m = re.exec(body)) !== null) {
        if (m.index > lastIndex) {
            parts.push({ type: 'text', text: body.slice(lastIndex, m.index) });
        }
        parts.push({ type: 'mention', idUser: Number(m[2]), name: m[1] });
        lastIndex = m.index + m[0].length;
    }
    if (lastIndex < body.length) {
        parts.push({ type: 'text', text: body.slice(lastIndex) });
    }
    return parts.length ? parts : [{ type: 'text', text: '' }];
}
