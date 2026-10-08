import { MarkedExtension } from 'marked';
import { MARKED_EXTENSIONS } from 'ngx-markdown';

const MENTION_AT_START_RE = /^@\[([^\]]+)\]\(user:(\d+)\)/;

const escapeHtml = (value: string): string =>
    value
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');

export const mentionExtension: MarkedExtension = {
    extensions: [
        {
            name: 'mention',
            level: 'inline',
            start: (src: string) => {
                const index = src.indexOf('@[');
                return index === -1 ? undefined : index;
            },
            tokenizer: (src: string) => {
                const match = MENTION_AT_START_RE.exec(src);
                return match
                    ? { type: 'mention', raw: match[0], name: match[1], idUser: Number(match[2]) }
                    : undefined;
            },
            renderer: token =>
                `<span class="mention-chip">@${escapeHtml(token['name'] as string)}</span>`
        }
    ]
};

export const MARKDOWN_MENTION_EXTENSION = {
    provide: MARKED_EXTENSIONS,
    useValue: mentionExtension,
    multi: true as const
};
