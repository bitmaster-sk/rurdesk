import { WikiAgentAccess } from '../constants/wiki-agent-access.enum';
import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';
import { WikiTree } from '../model/wiki-tree.model';
import { WikiLinkContext, WikiLinkConverter } from './wiki-link.converter';

const context: WikiLinkContext = {
    idProject: 7,
    sharedIdSpace: 1,
    links: [
        {
            shared: false,
            slug: 'prehlad-systemu',
            idPage: 10,
            idSpace: 2,
            title: 'Prehľad systému'
        },
        { shared: false, slug: 'commits', idPage: 11, idSpace: 1, title: 'Ako píšeme commity' },
        { shared: true, slug: 'security', idPage: 12, idSpace: 1, title: 'Security' },
        { shared: false, slug: 'missing-page', idPage: null, idSpace: 2, title: '' }
    ]
};

describe('WikiLinkConverter', () => {
    it('turns a resolved link into a route of its space using the page title', () => {
        expect(WikiLinkConverter.toMarkdown('See [[Prehľad systému]].', context)).toBe(
            'See [Prehľad systému](/project/7/wiki/project/prehlad-systemu).'
        );
    });

    it('routes a link that fell back to a shared page into the shared space', () => {
        expect(WikiLinkConverter.toMarkdown('[[commits]]', context)).toBe(
            '[Ako píšeme commity](/project/7/wiki/instance/commits)'
        );
        expect(WikiLinkConverter.toMarkdown('[[shared:security]]', context)).toBe(
            '[Security](/project/7/wiki/instance/security)'
        );
    });

    it('keeps an explicit label', () => {
        expect(WikiLinkConverter.toMarkdown('[[prehlad-systemu|the overview]]', context)).toBe(
            '[the overview](/project/7/wiki/project/prehlad-systemu)'
        );
    });

    it('points a missing page to the create form', () => {
        expect(WikiLinkConverter.toMarkdown('[[Missing page]]', context)).toBe(
            '[Missing page](/project/7/wiki/new?space=project&title=Missing%20page "missing")'
        );
    });

    it('turns checklist markers into boxes the sanitizer keeps', () => {
        expect(
            WikiLinkConverter.toMarkdown('- [ ] open\n  - [x] done\n`- [ ] code`', context)
        ).toBe(
            '- <span class="wiki-task"></span> open\n  - <span class="wiki-task wiki-task--done"></span> done\n`- [ ] code`'
        );
    });

    it('links task references but not headings', () => {
        expect(WikiLinkConverter.toMarkdown('# Title\nfixed in #11 (and #12)', context)).toBe(
            '# Title\nfixed in [#11](/project/7/issue/11) (and [#12](/project/7/issue/12))'
        );
    });

    it('leaves links and task references inside code untouched', () => {
        const body = 'Use `[[prehlad-systemu]]` and\n```\n[[prehlad-systemu]] #5\n```';
        expect(WikiLinkConverter.toMarkdown(body, context)).toBe(body);
    });

    it('parses targets with the shared prefix and a label', () => {
        expect(WikiLinkConverter.toTarget('spolocne:Ako píšeme commity|commits')).toEqual({
            shared: true,
            slug: 'ako-piseme-commity',
            label: 'commits',
            page: 'Ako píšeme commity',
            heading: '',
            anchor: ''
        });
        expect(WikiLinkConverter.toTarget('  ')).toBeNull();
    });

    it('links to a heading on another page, keeping the page title in the label', () => {
        expect(WikiLinkConverter.toMarkdown('[[Prehľad systému#Vrstvy aplikácie]]', context)).toBe(
            '[Prehľad systému › Vrstvy aplikácie](/project/7/wiki/project/prehlad-systemu#vrstvy-aplikacie)'
        );
        expect(WikiLinkConverter.toMarkdown('[[prehlad-systemu#Vrstvy|layers]]', context)).toBe(
            '[layers](/project/7/wiki/project/prehlad-systemu#vrstvy)'
        );
    });

    it('links to a heading on the same page', () => {
        expect(WikiLinkConverter.toMarkdown('see [[#Rollback plán]]', context)).toBe(
            'see [Rollback plán](#rollback-plan)'
        );
    });

    it('does not turn a numeric heading anchor into a task link', () => {
        expect(WikiLinkConverter.toMarkdown('[[#2026]] and [year](#2026)', context)).toBe(
            '[2026](#2026) and [year](#2026)'
        );
    });

    it('links only wiki pages in a comment and leaves task refs, checklists and code alone', () => {
        expect(
            WikiLinkConverter.toPageLinksMarkdown(
                'see [[Prehľad systému]] for #11\n- [ ] todo\n`[[commits]]`',
                context
            )
        ).toBe(
            'see [Prehľad systému](/project/7/wiki/project/prehlad-systemu) for #11\n- [ ] todo\n`[[commits]]`'
        );
    });

    it('resolves a comment link like the server: project first, then the shared space', () => {
        const node = (idPage: number, idSpace: number, slug: string, title: string) => ({
            idPage,
            idSpace,
            idParent: null,
            slug,
            title,
            agentAccess: WikiAgentAccess.OnDemand,
            rank: 'm'
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
                node(10, 2, 'deploy', 'Deploy here'),
                node(11, 1, 'deploy', 'Deploy everywhere'),
                node(12, 1, 'security', 'Security')
            ],
            alwaysTokens: 0,
            tokenLimit: 0,
            trashCount: 0
        };
        const treeContext = WikiLinkConverter.toContext(7, tree);
        expect(
            WikiLinkConverter.toPageLinksMarkdown(
                '[[deploy]] [[shared:deploy]] [[security]] [[nope]]',
                treeContext
            )
        ).toBe(
            '[Deploy here](/project/7/wiki/project/deploy) ' +
                '[Deploy everywhere](/project/7/wiki/instance/deploy) ' +
                '[Security](/project/7/wiki/instance/security) ' +
                '[nope](/project/7/wiki/new?space=project&title=nope "missing")'
        );
    });

    it('writes a picked page as a link, with the shared prefix and a slug for awkward titles', () => {
        expect(
            WikiLinkConverter.toLinkText({
                idPage: 1,
                title: 'Deploy runbook',
                slug: 'deploy-runbook',
                kind: WikiSpaceKind.Project
            })
        ).toBe('[[Deploy runbook]]');
        expect(
            WikiLinkConverter.toLinkText({
                idPage: 2,
                title: 'A | B [draft]',
                slug: 'a-b-draft',
                kind: WikiSpaceKind.Instance
            })
        ).toBe('[[shared:a-b-draft]]');
    });
});
