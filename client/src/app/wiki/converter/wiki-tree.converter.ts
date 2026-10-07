import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiLinkCandidate } from '../entity/wiki-link-candidate.entity';
import { WikiPagePickGroup } from '../entity/wiki-page-pick.entity';
import { WikiParentOption } from '../entity/wiki-parent-option.entity';
import { WikiTreeEntry, WikiTreeGroups } from '../entity/wiki-tree-entry.entity';
import { WikiTree, WikiTreeNode } from '../model/wiki-tree.model';
import { WikiSlugConverter } from './wiki-slug.converter';

export abstract class WikiTreeConverter {
    public static toGroups(tree: WikiTree): WikiTreeGroups {
        const shared = tree.spaces.find(space => space.kind === WikiSpaceKind.Instance);
        const project = tree.spaces.find(space => space.kind === WikiSpaceKind.Project);
        const always = tree.nodes
            .filter(node => node.agentAccess === WikiAgentAccess.Always)
            .sort((left, right) => left.title.localeCompare(right.title));
        return {
            always,
            shared: shared ? WikiTreeConverter.toEntries(tree.nodes, shared.idSpace) : [],
            project: project ? WikiTreeConverter.toEntries(tree.nodes, project.idSpace) : []
        };
    }

    public static toEntries(nodes: WikiTreeNode[], idSpace: number): WikiTreeEntry[] {
        const inSpace = nodes.filter(node => node.idSpace === idSpace);
        const ids = new Set(inSpace.map(node => node.idPage));
        const byParent = new Map<number | null, WikiTreeNode[]>();
        for (const node of inSpace) {
            const parent = node.idParent !== null && ids.has(node.idParent) ? node.idParent : null;
            const siblings = byParent.get(parent) ?? [];
            siblings.push(node);
            byParent.set(parent, siblings);
        }
        const build = (idParent: number | null): WikiTreeEntry[] =>
            (byParent.get(idParent) ?? [])
                .sort((left, right) =>
                    left.rank === right.rank
                        ? left.idPage - right.idPage
                        : left.rank < right.rank
                          ? -1
                          : 1
                )
                .map(node => {
                    const children = build(node.idPage);
                    const descendants = children.reduce(
                        (sum, child) => sum + 1 + child.descendants,
                        0
                    );
                    return { node, children, descendants };
                });
        return build(null);
    }

    public static ancestorIds(nodes: WikiTreeNode[], idPage: number): number[] {
        const byId = new Map(nodes.map(node => [node.idPage, node]));
        const ancestors: number[] = [];
        let idParent = byId.get(idPage)?.idParent ?? null;
        while (idParent !== null && !ancestors.includes(idParent)) {
            ancestors.push(idParent);
            idParent = byId.get(idParent)?.idParent ?? null;
        }
        return ancestors;
    }

    public static subtreeIds(nodes: WikiTreeNode[], idPage: number): number[] {
        const result = [idPage];
        for (let index = 0; index < result.length; index++) {
            for (const node of nodes) {
                if (node.idParent === result[index] && !result.includes(node.idPage)) {
                    result.push(node.idPage);
                }
            }
        }
        return result;
    }

    public static toParentOptions(
        nodes: WikiTreeNode[],
        idSpace: number,
        rootLabel: string,
        idsExcluded: number[] = []
    ): WikiParentOption[] {
        const excluded = new Set(idsExcluded);
        const options: WikiParentOption[] = [{ label: rootLabel, value: null, depth: 0 }];
        const walk = (entries: WikiTreeEntry[], depth: number): void => {
            for (const entry of entries) {
                if (excluded.has(entry.node.idPage)) {
                    continue;
                }
                options.push({ label: entry.node.title, value: entry.node.idPage, depth });
                walk(entry.children, depth + 1);
            }
        };
        walk(WikiTreeConverter.toEntries(nodes, idSpace), 0);
        return options;
    }

    public static toPickGroups(
        tree: WikiTree,
        query: string,
        idsLinked: number[]
    ): WikiPagePickGroup[] {
        const linked = new Set(idsLinked);
        const words = WikiTreeConverter.toQueryWords(query);
        return [WikiSpaceKind.Instance, WikiSpaceKind.Project]
            .map(kind => {
                const space = tree.spaces.find(candidate => candidate.kind === kind);
                const options = space
                    ? WikiTreeConverter.toParentOptions(tree.nodes, space.idSpace, '').slice(1)
                    : [];
                const pages = options
                    .filter(option => {
                        const title = WikiSlugConverter.toSlug(option.label);
                        return words.every(word => title.includes(word));
                    })
                    .map(option => ({
                        idPage: option.value ?? 0,
                        title: option.label,
                        depth: words.length > 0 ? 0 : option.depth,
                        isLinked: linked.has(option.value ?? 0)
                    }));
                return { kind, pages };
            })
            .filter(group => group.pages.length > 0);
    }

    public static toLinkCandidates(
        tree: WikiTree,
        query: string,
        limit: number
    ): WikiLinkCandidate[] {
        const words = WikiTreeConverter.toQueryWords(query);
        const kindOf = new Map(tree.spaces.map(space => [space.idSpace, space.kind]));
        return [WikiSpaceKind.Project, WikiSpaceKind.Instance]
            .flatMap(kind =>
                tree.nodes
                    .filter(node => kindOf.get(node.idSpace) === kind)
                    .filter(node => WikiTreeConverter.isTitleMatch(node.title, words))
                    .sort((left, right) => left.title.localeCompare(right.title))
                    .map(node => ({
                        idPage: node.idPage,
                        title: node.title,
                        slug: node.slug,
                        kind
                    }))
            )
            .slice(0, limit);
    }

    private static toQueryWords(query: string): string[] {
        return WikiSlugConverter.toSlug(query).split('-').filter(Boolean);
    }

    private static isTitleMatch(title: string, words: string[]): boolean {
        const slug = WikiSlugConverter.toSlug(title);
        return words.every(word => slug.includes(word));
    }
}
