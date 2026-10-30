import { IssueLinkPart } from '../entity/issue-link-part.entity';
import { IssueLinkRef } from '../model/issue-link.model';

export abstract class IssueLinkConverter {
    private static readonly linkPattern = /\[#(\d+)\]\(\/project\/(\d+)\/issue\/(\d+)\)/g;
    private static readonly hrefPattern = /^\/project\/(\d+)\/issue\/(\d+)$/;

    public static toLinkMarkdown(ref: IssueLinkRef): string {
        return `[#${ref.idIssuePublic}](${IssueLinkConverter.toHref(ref)})`;
    }

    public static toHref(ref: IssueLinkRef): string {
        return `/project/${ref.idProject}/issue/${ref.idIssuePublic}`;
    }

    public static toKey(ref: IssueLinkRef): string {
        return `${ref.idProject}/${ref.idIssuePublic}`;
    }

    public static toRef(href: string): IssueLinkRef | null {
        const match = IssueLinkConverter.hrefPattern.exec(href);
        return match ? { idProject: Number(match[1]), idIssuePublic: Number(match[2]) } : null;
    }

    public static toParts(text: string): IssueLinkPart[] {
        const parts: IssueLinkPart[] = [];
        const pattern = new RegExp(IssueLinkConverter.linkPattern.source, 'g');
        let lastIndex = 0;
        let match: RegExpExecArray | null;
        while ((match = pattern.exec(text)) !== null) {
            if (match[1] !== match[3]) {
                continue;
            }
            if (match.index > lastIndex) {
                parts.push({ type: 'text', text: text.slice(lastIndex, match.index) });
            }
            parts.push({
                type: 'issue',
                idProject: Number(match[2]),
                idIssuePublic: Number(match[3])
            });
            lastIndex = match.index + match[0].length;
        }
        if (lastIndex < text.length) {
            parts.push({ type: 'text', text: text.slice(lastIndex) });
        }
        return parts;
    }
}
