import { MarkdownCode } from 'src/app/shared/markdown/markdown-code';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiLinkCandidate } from '../entity/wiki-link-candidate.entity';
import { WikiResolvedLink } from '../model/wiki-page.model';
import { WikiTree } from '../model/wiki-tree.model';
import { WikiSlugConverter } from './wiki-slug.converter';

export interface WikiLinkTarget {
    shared: boolean;
    slug: string;
    label: string;
    page: string;
    heading: string;
    anchor: string;
}

export interface WikiLinkContext {
    idProject: number;
    links: WikiResolvedLink[];
    sharedIdSpace: number | null;
}

export abstract class WikiLinkConverter {
    private static readonly sharedPrefixes = ['shared:', 'spolocne:'];
    private static readonly linkPattern = /\[\[([^[\]\n]+?)\]\]/g;
    private static readonly issuePattern = /(^|[\s(])(?<!\]\()#(\d+)\b/g;
    private static readonly taskPattern = /^(\s*(?:[-*+]|\d+[.)]) )\[([ xX])\] /gm;

    public static toTarget(raw: string): WikiLinkTarget | null {
        const [targetPart, labelPart] = raw.split('|', 2);
        const hashAt = targetPart.indexOf('#');
        const heading = hashAt === -1 ? '' : targetPart.slice(hashAt + 1).trim();
        let target = (hashAt === -1 ? targetPart : targetPart.slice(0, hashAt)).trim();
        let shared = false;
        const lowered = target.toLowerCase();
        for (const prefix of WikiLinkConverter.sharedPrefixes) {
            if (lowered.startsWith(prefix)) {
                shared = true;
                target = target.slice(prefix.length);
                break;
            }
        }
        const slug = WikiSlugConverter.toSlug(target);
        const anchor = WikiSlugConverter.toSlug(heading);
        if (!slug && !anchor) {
            return null;
        }
        const page = target.trim();
        const defaultLabel = page && heading ? `${page} › ${heading}` : page || heading;
        return {
            shared,
            slug,
            label: (labelPart ?? defaultLabel).trim(),
            page,
            heading,
            anchor
        };
    }

    public static toMarkdown(body: string, context: WikiLinkContext): string {
        return MarkdownCode.replaceOutsideCode(body, part =>
            WikiLinkConverter.replaceOutsideCode(part, context)
        );
    }

    public static toPageLinksMarkdown(body: string, context: WikiLinkContext): string {
        return MarkdownCode.replaceOutsideCode(body, part =>
            WikiLinkConverter.replaceLinks(part, context)
        );
    }

    public static toContext(idProject: number, tree: WikiTree): WikiLinkContext {
        const shared = tree.spaces.find(space => space.kind === WikiSpaceKind.Instance);
        const project = tree.spaces.find(space => space.kind === WikiSpaceKind.Project);
        const sharedNodes = tree.nodes.filter(node => node.idSpace === shared?.idSpace);
        const projectNodes = tree.nodes.filter(node => node.idSpace === project?.idSpace);
        const projectSlugs = new Set(projectNodes.map(node => node.slug));
        const toLink = (
            node: { idPage: number; idSpace: number; slug: string; title: string },
            isShared: boolean
        ): WikiResolvedLink => ({
            shared: isShared,
            slug: node.slug,
            idPage: node.idPage,
            idSpace: node.idSpace,
            title: node.title
        });
        return {
            idProject,
            sharedIdSpace: shared?.idSpace ?? null,
            links: [
                ...projectNodes.map(node => toLink(node, false)),
                ...sharedNodes.map(node => toLink(node, true)),
                ...sharedNodes
                    .filter(node => !projectSlugs.has(node.slug))
                    .map(node => toLink(node, false))
            ]
        };
    }

    public static toLinkText(candidate: WikiLinkCandidate): string {
        const target = /[[\]|#]/.test(candidate.title) ? candidate.slug : candidate.title;
        const prefix = candidate.kind === WikiSpaceKind.Instance ? 'shared:' : '';
        return `[[${prefix}${target}]]`;
    }

    private static replaceOutsideCode(text: string, context: WikiLinkContext): string {
        const withTasks = text.replace(
            WikiLinkConverter.taskPattern,
            (_match, marker: string, state: string) =>
                `${marker}<span class="wiki-task${state === ' ' ? '' : ' wiki-task--done'}"></span> `
        );
        return WikiLinkConverter.replaceLinks(withTasks, context).replace(
            WikiLinkConverter.issuePattern,
            (_match, lead: string, id: string) =>
                `${lead}[#${id}](/project/${context.idProject}/issue/${id})`
        );
    }

    private static replaceLinks(text: string, context: WikiLinkContext): string {
        return text.replace(WikiLinkConverter.linkPattern, (match, raw: string) => {
            const target = WikiLinkConverter.toTarget(raw);
            if (!target) {
                return match;
            }
            const fragment = target.anchor ? `#${target.anchor}` : '';
            if (!target.slug) {
                return `[${WikiLinkConverter.escapeLabel(target.label)}](${fragment})`;
            }
            const resolved = context.links.find(
                link => link.slug === target.slug && link.shared === target.shared
            );
            const resolvedLabel =
                resolved?.title && target.heading
                    ? `${resolved.title} › ${target.heading}`
                    : resolved?.title || target.label;
            const label = WikiLinkConverter.escapeLabel(
                raw.includes('|') ? target.label : resolvedLabel
            );
            if (!resolved?.idPage) {
                const space = target.shared ? WikiSpaceKind.Instance : WikiSpaceKind.Project;
                const title = encodeURIComponent(target.page);
                return `[${label}](/project/${context.idProject}/wiki/new?space=${space}&title=${title} "missing")`;
            }
            const space =
                context.sharedIdSpace !== null && resolved.idSpace === context.sharedIdSpace
                    ? WikiSpaceKind.Instance
                    : WikiSpaceKind.Project;
            return `[${label}](/project/${context.idProject}/wiki/${space}/${resolved.slug}${fragment})`;
        });
    }

    private static escapeLabel(label: string): string {
        return label.replace(/([[\]\\])/g, '\\$1');
    }
}
