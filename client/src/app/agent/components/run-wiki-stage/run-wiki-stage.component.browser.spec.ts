import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { beforeEach, describe, expect, it } from 'vitest';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { TablerIconStub } from 'src/testing/stubs';
import { RunWikiContextConverter } from '../../converter/run-wiki-context.converter';
import { AgentRunWikiRead } from '../../model/agent-run-wiki-read.model';
import { AgentWikiReadSource } from '../../model/agent-wiki-read-source.enum';
import { RunWikiStageComponent } from './run-wiki-stage.component';

function wikiRead(overrides: Partial<AgentRunWikiRead>): AgentRunWikiRead {
    return {
        idRead: 1,
        idRun: 5,
        idTask: 50,
        stage: 'design',
        idCall: 'prompt',
        source: AgentWikiReadSource.PromptAlways,
        idPage: 10,
        spaceKind: WikiSpaceKind.Project,
        slug: 'how-we-work',
        title: 'How we work',
        versionNo: 3,
        tokens: 120,
        query: null,
        pages: null,
        createAt: '2026-10-07T10:00:00Z',
        ...overrides
    };
}

const READS: AgentRunWikiRead[] = [
    wikiRead({}),
    wikiRead({
        idRead: 2,
        source: AgentWikiReadSource.PromptLinked,
        idPage: 11,
        slug: 'gateway',
        title: 'Gateway',
        versionNo: 4
    }),
    wikiRead({
        idRead: 3,
        source: AgentWikiReadSource.PromptIndex,
        idPage: null,
        slug: null,
        title: null,
        versionNo: null,
        spaceKind: null,
        pages: [
            {
                idPage: 12,
                spaceKind: WikiSpaceKind.Project,
                slug: 'runbook',
                title: 'Deploy runbook'
            }
        ]
    }),
    wikiRead({
        idRead: 4,
        source: AgentWikiReadSource.McpSearch,
        idCall: 's1',
        query: 'worktree retention',
        idPage: 13,
        slug: 'retention',
        title: 'Worktree retention'
    }),
    wikiRead({
        idRead: 5,
        source: AgentWikiReadSource.McpGet,
        idCall: 'g1',
        idPage: 14,
        slug: 'security',
        spaceKind: WikiSpaceKind.Instance,
        title: 'Security',
        versionNo: 1,
        tokens: 1840
    })
];

describe('RunWikiStageComponent', () => {
    let fixture: ComponentFixture<RunWikiStageComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), UiModule, TablerIconStub, RouterModule],
            declarations: [RunWikiStageComponent],
            providers: [provideRouter([])]
        }).compileComponents();
        fixture = TestBed.createComponent(RunWikiStageComponent);
    });

    function render(reads: AgentRunWikiRead[] = READS): HTMLElement {
        const [stage] = RunWikiContextConverter.toStages(reads, 7, ['design']);
        fixture.componentRef.setInput('stage', stage);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function texts(el: HTMLElement, testId: string): string[] {
        return Array.from(el.querySelectorAll(`[data-testid="${testId}"]`)).map(
            node => node.textContent?.replace(/\s+/g, ' ').trim() ?? ''
        );
    }

    function toggle(el: HTMLElement, testId: string): void {
        (el.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement).click();
        fixture.detectChanges();
    }

    it('sums up the wiki of the stage in one line of sections', () => {
        const el = render();

        expect(texts(el, 'run-wiki-prompt-toggle')).toEqual(['2 AGENT.WIKI.PROMPT_SEGMENT']);
        expect(texts(el, 'run-wiki-calls-toggle')).toEqual(['2 AGENT.WIKI.MCP_SEGMENT']);
        expect(texts(el, 'run-wiki-index')).toEqual(['1 AGENT.WIKI.INDEX_SEGMENT']);
    });

    it('opens the pages sent in the prompt by default', () => {
        const el = render();

        const pages = texts(el, 'run-wiki-prompt-page');
        expect(pages).toHaveLength(2);
        expect(pages[0]).toContain('How we work');
        expect(pages[0]).toContain('v3');
        expect(pages[0]).toContain('AGENT.WIKI.ALWAYS');
        expect(pages[1]).toContain('Gateway');
        expect(pages[1]).toContain('v4');
        expect(pages[1]).toContain('AGENT.WIKI.LINKED');
        expect(el.textContent).toContain('AGENT.WIKI.PROMPT_HINT');
        expect(texts(el, 'run-wiki-call')).toEqual([]);
    });

    it('shows what the agent looked up once the reader opens that section', () => {
        const el = render();

        toggle(el, 'run-wiki-calls-toggle');

        const calls = texts(el, 'run-wiki-call');
        expect(calls[0]).toContain('AGENT.WIKI.SEARCHED');
        expect(calls[0]).toContain('Worktree retention');
        expect(calls[1]).toContain('Security');
        expect(calls[1]).toContain('v1');
        expect(texts(el, 'run-wiki-looked-up')[0]).toContain('AGENT.WIKI.MCP_HINT');
    });

    it('heads every open section with its count and the tokens it cost', () => {
        const el = render();
        toggle(el, 'run-wiki-calls-toggle');
        toggle(el, 'run-wiki-index');

        expect(texts(el, 'run-wiki-section-stats')).toEqual([
            'AGENT.WIKI.PAGES.PLURAL · AGENT.WIKI.TOKENS',
            'AGENT.WIKI.LOOKUPS.PLURAL · AGENT.WIKI.TOKENS',
            'AGENT.WIKI.PAGES.SINGULAR · AGENT.WIKI.TOKENS'
        ]);
    });

    it('lists the pages offered by title only after the reader opens them', () => {
        const el = render();
        expect(el.textContent).not.toContain('Deploy runbook');

        toggle(el, 'run-wiki-index');

        expect(texts(el, 'run-wiki-index-list')[0]).toContain('Deploy runbook');
        expect(el.textContent).toContain('AGENT.WIKI.INDEX_HINT');
    });

    it('closes a section the reader opened', () => {
        const el = render();

        toggle(el, 'run-wiki-prompt-toggle');

        expect(texts(el, 'run-wiki-prompt-page')).toEqual([]);
    });

    it('links every page into the wiki of the run project', () => {
        const el = render();
        toggle(el, 'run-wiki-calls-toggle');
        toggle(el, 'run-wiki-index');
        const hrefs = Array.from(el.querySelectorAll('a')).map(link => link.getAttribute('href'));

        expect(hrefs).toEqual([
            '/project/7/wiki/project/how-we-work',
            '/project/7/wiki/project/gateway',
            '/project/7/wiki/project/retention',
            '/project/7/wiki/instance/security',
            '/project/7/wiki/project/runbook'
        ]);
    });
});
