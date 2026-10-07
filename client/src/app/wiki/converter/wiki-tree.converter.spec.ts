import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiTree, WikiTreeNode } from '../model/wiki-tree.model';
import { WikiTreeConverter } from './wiki-tree.converter';

const node = (
    idPage: number,
    idSpace: number,
    idParent: number | null,
    rank: string,
    agentAccess = WikiAgentAccess.OnDemand
): WikiTreeNode => ({
    idPage,
    idSpace,
    idParent,
    slug: `p${idPage}`,
    title: `Page ${idPage}`,
    agentAccess,
    rank
});

const tree: WikiTree = {
    spaces: [
        {
            idSpace: 1,
            kind: WikiSpaceKind.Instance,
            canEdit: true,
            canManage: false,
            idHomePage: null
        },
        {
            idSpace: 2,
            kind: WikiSpaceKind.Project,
            canEdit: true,
            canManage: true,
            idHomePage: null
        }
    ],
    nodes: [
        node(1, 2, null, 'n'),
        node(2, 2, null, 'm'),
        node(3, 2, 1, 'm', WikiAgentAccess.Always),
        node(4, 2, 3, 'm'),
        node(5, 1, null, 'm'),
        node(6, 2, 99, 'z')
    ],
    alwaysTokens: 0,
    tokenLimit: 12000,
    trashCount: 0
};

describe('WikiTreeConverter', () => {
    it('splits nodes into always-read, shared and project trees ordered by rank', () => {
        const groups = WikiTreeConverter.toGroups(tree);
        expect(groups.always.map(entry => entry.idPage)).toEqual([3]);
        expect(groups.shared.map(entry => entry.node.idPage)).toEqual([5]);
        expect(groups.project.map(entry => entry.node.idPage)).toEqual([2, 1, 6]);
    });

    it('nests children and counts all descendants', () => {
        const page1 = WikiTreeConverter.toGroups(tree).project[1];
        expect(page1.children[0].node.idPage).toBe(3);
        expect(page1.children[0].children[0].node.idPage).toBe(4);
        expect(page1.descendants).toBe(2);
    });

    it('treats a node with an unknown parent as a root', () => {
        expect(WikiTreeConverter.toGroups(tree).project[2].node.idPage).toBe(6);
    });

    it('lists ancestors from the closest up and the whole subtree', () => {
        expect(WikiTreeConverter.ancestorIds(tree.nodes, 4)).toEqual([3, 1]);
        expect(WikiTreeConverter.subtreeIds(tree.nodes, 1)).toEqual([1, 3, 4]);
    });

    it('lists parent choices as plain titles with their depth and skips excluded pages', () => {
        const options = WikiTreeConverter.toParentOptions(tree.nodes, 2, 'Top', [4]);
        expect(options[0]).toEqual({ label: 'Top', value: null, depth: 0 });
        expect(options.every(option => !option.label.includes('—'))).toBe(true);
        expect(options.some(option => option.value === 4)).toBe(false);
        expect(options.find(option => option.value === 3)?.depth).toBe(1);
    });

    it('offers the pages of both spaces as an indented tree and marks linked ones', () => {
        const groups = WikiTreeConverter.toPickGroups(tree, '', [3]);
        expect(groups.map(group => group.kind)).toEqual([
            WikiSpaceKind.Instance,
            WikiSpaceKind.Project
        ]);
        const project = groups[1].pages;
        expect(project.map(page => [page.idPage, page.depth])).toEqual([
            [2, 0],
            [1, 0],
            [3, 1],
            [4, 2],
            [6, 0]
        ]);
        expect(project.find(page => page.idPage === 3)?.isLinked).toBe(true);
        expect(project.find(page => page.idPage === 4)?.isLinked).toBe(false);
    });

    it('filters picks by part of the title, ignoring case and diacritics', () => {
        const named: WikiTree = {
            ...tree,
            nodes: [
                { ...node(1, 2, null, 'm'), title: 'Prehľad systému' },
                { ...node(2, 2, 1, 'm'), title: 'Nasadenie' },
                { ...node(3, 1, null, 'm'), title: 'Security' }
            ]
        };
        const groups = WikiTreeConverter.toPickGroups(named, 'sys PREHL', []);
        expect(groups).toEqual([
            {
                kind: WikiSpaceKind.Project,
                pages: [{ idPage: 1, title: 'Prehľad systému', depth: 0, isLinked: false }]
            }
        ]);
        expect(WikiTreeConverter.toPickGroups(named, 'nothing', [])).toEqual([]);
    });

    it('suggests link targets by part of the title, project pages first, up to a limit', () => {
        const named: WikiTree = {
            ...tree,
            nodes: [
                { ...node(1, 1, null, 'm'), title: 'Deploy everywhere' },
                { ...node(2, 2, null, 'm'), title: 'Release' },
                { ...node(3, 2, 2, 'm'), title: 'Deploy here' }
            ]
        };
        expect(WikiTreeConverter.toLinkCandidates(named, 'depl', 8)).toEqual([
            { idPage: 3, title: 'Deploy here', slug: 'p3', kind: WikiSpaceKind.Project },
            { idPage: 1, title: 'Deploy everywhere', slug: 'p1', kind: WikiSpaceKind.Instance }
        ]);
        expect(WikiTreeConverter.toLinkCandidates(named, '', 2)).toHaveLength(2);
    });
});
