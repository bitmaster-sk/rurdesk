import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { RunWikiCall, RunWikiPage, RunWikiStage } from '../entity/run-wiki-context.entity';
import { AgentRunWikiIndexPage, AgentRunWikiRead } from '../model/agent-run-wiki-read.model';
import { AgentWikiReadSource } from '../model/agent-wiki-read-source.enum';

const PROMPT_PAGE_SOURCES = new Set([
    AgentWikiReadSource.PromptAlways,
    AgentWikiReadSource.PromptLinked
]);

export abstract class RunWikiContextConverter {
    public static toStages(
        reads: AgentRunWikiRead[],
        idProject: number,
        stageOrder: string[]
    ): RunWikiStage[] {
        const byStage = new Map<string, AgentRunWikiRead[]>();
        for (const read of reads) {
            byStage.set(read.stage, [...(byStage.get(read.stage) ?? []), read]);
        }
        return RunWikiContextConverter.orderStages([...byStage.keys()], stageOrder).map(stage =>
            RunWikiContextConverter.toStage(
                stage,
                RunWikiContextConverter.latestAttempt(byStage.get(stage) ?? []),
                idProject
            )
        );
    }

    public static toTarget(read: AgentRunWikiRead): string {
        if (read.source === AgentWikiReadSource.McpSearch) {
            return read.query ?? '';
        }
        const slug = read.slug ?? '';
        return read.spaceKind === WikiSpaceKind.Instance ? `shared:${slug}` : slug;
    }

    private static toStage(
        stage: string,
        reads: AgentRunWikiRead[],
        idProject: number
    ): RunWikiStage {
        const calls = new Map<string, RunWikiCall>();
        for (const read of reads) {
            if (
                read.source !== AgentWikiReadSource.McpGet &&
                read.source !== AgentWikiReadSource.McpSearch
            ) {
                continue;
            }
            const call = calls.get(read.idCall) ?? {
                idCall: read.idCall,
                isSearch: read.source === AgentWikiReadSource.McpSearch,
                target: RunWikiContextConverter.toTarget(read),
                pages: []
            };
            if (read.slug !== null) {
                call.pages.push(RunWikiContextConverter.toPage(read, idProject));
            }
            calls.set(read.idCall, call);
        }
        return {
            stage,
            prompt: reads
                .filter(read => PROMPT_PAGE_SOURCES.has(read.source))
                .map(read => ({
                    ...RunWikiContextConverter.toPage(read, idProject),
                    isAlways: read.source === AgentWikiReadSource.PromptAlways
                })),
            index: reads
                .filter(read => read.source === AgentWikiReadSource.PromptIndex)
                .flatMap(read => read.pages ?? [])
                .map(page => RunWikiContextConverter.toIndexPage(page, idProject)),
            calls: [...calls.values()]
        };
    }

    private static toPage(read: AgentRunWikiRead, idProject: number): RunWikiPage {
        const canOpen = read.idPage !== null && read.spaceKind !== null && read.slug !== null;
        return {
            key: read.idRead,
            title: read.title ?? read.slug ?? '',
            versionNo: read.versionNo,
            tokens: read.tokens,
            link: canOpen
                ? ['/project', idProject, 'wiki', read.spaceKind as string, read.slug as string]
                : null
        };
    }

    private static toIndexPage(page: AgentRunWikiIndexPage, idProject: number): RunWikiPage {
        return {
            key: page.idPage,
            title: page.title,
            versionNo: null,
            tokens: 0,
            link: ['/project', idProject, 'wiki', page.spaceKind, page.slug]
        };
    }

    private static latestAttempt(reads: AgentRunWikiRead[]): AgentRunWikiRead[] {
        const idsTask = reads
            .map(read => read.idTask)
            .filter((idTask): idTask is number => idTask !== null);
        if (idsTask.length === 0) {
            return reads;
        }
        const latest = Math.max(...idsTask);
        return reads.filter(read => read.idTask === latest);
    }

    private static orderStages(stages: string[], stageOrder: string[]): string[] {
        const rank = (stage: string): number => {
            const index = stageOrder.indexOf(stage);
            return index === -1 ? stageOrder.length : index;
        };
        return [...stages].sort((a, b) => rank(a) - rank(b));
    }
}
