import { MarkdownCode } from 'src/app/shared/markdown/markdown-code';
import { AttachmentLinkPart } from '../entity/attachment-link-part.entity';

export abstract class AttachmentLink {
    public static readonly pattern = /(!?)\[([^\]\n]*)\]\(attachment:([0-9a-fA-F-]{36})\)/g;

    public static hasAny(text: string): boolean {
        const re = new RegExp(AttachmentLink.pattern.source);
        return MarkdownCode.outsideCode(text).some(part => re.test(part));
    }

    public static parse(text: string): AttachmentLinkPart[] {
        const parts: AttachmentLinkPart[] = [];
        const re = new RegExp(AttachmentLink.pattern.source, 'g');
        let lastIndex = 0;
        let m: RegExpExecArray | null;
        while ((m = re.exec(text)) !== null) {
            if (m.index > lastIndex) {
                parts.push({ type: 'text', text: text.slice(lastIndex, m.index) });
            }
            parts.push({
                type: 'attachment',
                markdown: m[0],
                name: m[2],
                idAttachment: m[3],
                isImage: m[1] === '!'
            });
            lastIndex = m.index + m[0].length;
        }
        if (lastIndex < text.length) {
            parts.push({ type: 'text', text: text.slice(lastIndex) });
        }
        return parts;
    }
}
