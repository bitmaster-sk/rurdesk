import { describe, expect, it } from 'vitest';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { AgentRunWikiRead } from '../model/agent-run-wiki-read.model';
import { AgentWikiReadSource } from '../model/agent-wiki-read-source.enum';
import { RunWikiContextConverter } from './run-wiki-context.converter';

let nextId = 1;

function read(overrides: Partial<AgentRunWikiRead>): AgentRunWikiRead {
    const idRead = nextId++;
    return {
        idRead,
        idRun: 1,
        idTask: 10,
        stage: 'design',
        idCall: 'prompt',
        source: AgentWikiReadSource.PromptAlways,
        idPage: idRead,
        spaceKind: WikiSpaceKind.Project,
        slug: `page-${idRead}`,
        title: `Page ${idRead}`,
        versionNo: 1,
        tokens: 100,
        query: null,
        pages: null,
        createAt: '2026-10-07T10:00:00Z',
        ...overrides
    };
}

const STAGES = ['brainstorming', 'design', 'implementation_plan', 'implementation'];

describe('RunWikiContextConverter', () => {
    it('lists prompt pages with their reason, counts the index and links into the wiki', () => {
        const stages = RunWikiContextConverter.toStages(
            [
                read({ title: 'How we work', slug: 'how-we-work', versionNo: 3 }),
                read({
                    source: AgentWikiReadSource.PromptLinked,
                    title: 'Security',
                    slug: 'security',
                    spaceKind: WikiSpaceKind.Instance
                }),
                read({
                    source: AgentWikiReadSource.PromptIndex,
                    idPage: null,
                    slug: null,
                    title: null,
                    versionNo: null,
                    spaceKind: null,
                    pages: [
                        {
                            idPage: 40,
                            spaceKind: WikiSpaceKind.Project,
                            slug: 'runbook',
                            title: 'Runbook'
                        },
                        {
                            idPage: 41,
                            spaceKind: WikiSpaceKind.Instance,
                            slug: 'retention',
                            title: 'Retention'
                        }
                    ]
                })
            ],
            7,
            STAGES
        );

        expect(stages).toHaveLength(1);
        expect(stages[0].stage).toBe('design');
        expect(
            stages[0].prompt.map(page => [page.title, page.versionNo, page.isAlways, page.link])
        ).toEqual([
            ['How we work', 3, true, ['/project', 7, 'wiki', 'project', 'how-we-work']],
            ['Security', 1, false, ['/project', 7, 'wiki', 'instance', 'security']]
        ]);
        expect(stages[0].index.map(page => [page.title, page.link])).toEqual([
            ['Runbook', ['/project', 7, 'wiki', 'project', 'runbook']],
            ['Retention', ['/project', 7, 'wiki', 'instance', 'retention']]
        ]);
        expect(stages[0].calls).toEqual([]);
    });

    it('groups MCP reads into calls, keeps an empty search and prefixes shared slugs', () => {
        const stages = RunWikiContextConverter.toStages(
            [
                read({ source: AgentWikiReadSource.McpSearch, idCall: 's1', query: 'worktree' }),
                read({ source: AgentWikiReadSource.McpSearch, idCall: 's1', query: 'worktree' }),
                read({
                    source: AgentWikiReadSource.McpSearch,
                    idCall: 's2',
                    query: 'nothing',
                    idPage: null,
                    slug: null,
                    title: null,
                    versionNo: null,
                    spaceKind: null
                }),
                read({
                    source: AgentWikiReadSource.McpGet,
                    idCall: 'g1',
                    slug: 'security',
                    spaceKind: WikiSpaceKind.Instance,
                    tokens: 1840
                })
            ],
            7,
            STAGES
        );

        const calls = stages[0].calls;
        expect(calls.map(call => [call.isSearch, call.target, call.pages.length])).toEqual([
            [true, 'worktree', 2],
            [true, 'nothing', 0],
            [false, 'shared:security', 1]
        ]);
        expect(calls[2].pages[0].tokens).toBe(1840);
    });

    it('shows only the latest attempt of each stage, in stage order', () => {
        const stages = RunWikiContextConverter.toStages(
            [
                read({ stage: 'implementation', idTask: 30, title: 'Impl' }),
                read({ stage: 'design', idTask: 10, title: 'Old design read' }),
                read({ stage: 'design', idTask: 11, title: 'New design read' })
            ],
            7,
            STAGES
        );

        expect(stages.map(stage => stage.stage)).toEqual(['design', 'implementation']);
        expect(stages[0].prompt.map(page => page.title)).toEqual(['New design read']);
    });

    it('does not link a page that was purged from the wiki', () => {
        const stages = RunWikiContextConverter.toStages(
            [read({ idPage: null, title: 'Gone' })],
            7,
            STAGES
        );

        expect(stages[0].prompt[0].link).toBeNull();
        expect(stages[0].prompt[0].title).toBe('Gone');
    });

    it('returns nothing for a run that read no wiki', () => {
        expect(RunWikiContextConverter.toStages([], 7, STAGES)).toEqual([]);
    });
});
