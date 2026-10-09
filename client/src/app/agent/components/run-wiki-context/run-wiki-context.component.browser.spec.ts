import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { UiModule } from 'src/app/ui/ui.module';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { TablerIconStub } from 'src/testing/stubs';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { AgentPhase } from '../../model/agent-phase.enum';
import { AgentRun } from '../../model/agent-run.model';
import { AgentRunWikiRead } from '../../model/agent-run-wiki-read.model';
import { AgentWikiReadSource } from '../../model/agent-wiki-read-source.enum';
import { RunWikiContextComponent } from './run-wiki-context.component';

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

function statsNotice(idRun: number): Notice<unknown> {
    return { subject: NoticeSubject.AgentStats, action: NoticeAction.Update, payload: { idRun } };
}

function run(overrides: Partial<AgentRun> = {}): AgentRun {
    return {
        idRun: 5,
        idIssue: 1,
        idProject: 7,
        idUserAgent: 3,
        idGitIntegration: null,
        phase: AgentPhase.InProgress,
        stagePlan: { stages: [{ name: 'design', skippable: false, skip: false }] },
        queuePosition: null,
        prUrl: null,
        prHostType: null,
        prId: null,
        branchName: null,
        errorMessage: null,
        startedAt: null,
        finishedAt: null,
        createdAt: '2026-10-07T10:00:00Z',
        stages: [{ stage: 'design', status: 'active', attemptNo: 1 }],
        ...overrides
    };
}

describe('RunWikiContextComponent', () => {
    let fixture: ComponentFixture<RunWikiContextComponent>;
    let loadWikiReads: ReturnType<typeof vi.fn>;
    let stats: Subject<Notice<unknown>>;

    beforeEach(async () => {
        loadWikiReads = vi.fn().mockReturnValue(of(READS));
        stats = new Subject<Notice<unknown>>();
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), UiModule, TablerIconStub, RouterModule],
            declarations: [RunWikiContextComponent],
            providers: [
                provideRouter([]),
                { provide: AgentRunApi, useValue: { loadWikiReads$: loadWikiReads } },
                { provide: NoticeService, useValue: { agentStats$: stats.asObservable() } }
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(RunWikiContextComponent);
    });

    function render(value: AgentRun): HTMLElement {
        fixture.componentRef.setInput('run', value);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function openStages(el: HTMLElement): void {
        el.querySelectorAll<HTMLButtonElement>('[data-testid="run-wiki-stage-toggle"]').forEach(
            toggle => toggle.click()
        );
        fixture.detectChanges();
    }

    function texts(el: HTMLElement, testId: string): string[] {
        return Array.from(el.querySelectorAll(`[data-testid="${testId}"]`)).map(
            node => node.textContent?.replace(/\s+/g, ' ').trim() ?? ''
        );
    }

    it('sums up each stage in one collapsed line', () => {
        const el = render(run());

        expect(loadWikiReads).toHaveBeenCalledWith(5);
        expect(texts(el, 'run-wiki-context')[0]).toContain('AGENT.WIKI.TITLE');
        const summary = texts(el, 'run-wiki-stage-toggle');
        expect(summary).toHaveLength(1);
        expect(summary[0]).toContain('AGENT.WIKI.IN_PROMPT');
        expect(summary[0]).toContain('AGENT.WIKI.IN_INDEX');
        expect(summary[0]).toContain('AGENT.WIKI.OPENED');
        expect(summary[0]).toContain('AGENT.WIKI.SEARCHES');
        expect(el.querySelector('[data-testid="run-wiki-prompt-page"]')).toBeNull();
    });

    it('shows the pages sent in the prompt, the index and what the agent looked up once the stage is opened', () => {
        const el = render(run());
        openStages(el);

        const pages = texts(el, 'run-wiki-prompt-page');
        expect(pages).toHaveLength(2);
        expect(pages[0]).toContain('How we work');
        expect(pages[0]).toContain('v3');
        expect(pages[0]).toContain('AGENT.WIKI.ALWAYS');
        expect(pages[1]).toContain('Gateway');
        expect(pages[1]).toContain('v4');
        expect(pages[1]).toContain('AGENT.WIKI.LINKED');
        expect(texts(el, 'run-wiki-index')).toEqual(['AGENT.WIKI.INDEX']);
        const calls = texts(el, 'run-wiki-call');
        expect(calls[0]).toContain('AGENT.WIKI.SEARCHED');
        expect(calls[0]).not.toContain('search_wiki');
        expect(calls[0]).toContain('Worktree retention');
        expect(calls[1]).toContain('Security');
        expect(calls[1]).not.toContain('get_wiki_page');
        expect(calls[1]).toContain('v1');
    });

    it('lists the index pages only after the reader opens the index', () => {
        const el = render(run());
        openStages(el);
        expect(el.querySelector('[data-testid="run-wiki-index-list"]')).toBeNull();
        expect(el.textContent).not.toContain('Deploy runbook');

        (el.querySelector('[data-testid="run-wiki-index"]') as HTMLButtonElement).click();
        fixture.detectChanges();

        expect(texts(el, 'run-wiki-index-list')[0]).toContain('Deploy runbook');
        expect(el.textContent).toContain('AGENT.WIKI.INDEX_HINT');
    });

    it('links every page into the wiki of the run project', () => {
        const el = render(run());
        openStages(el);
        const hrefs = Array.from(el.querySelectorAll('a')).map(link => link.getAttribute('href'));

        expect(hrefs).toEqual([
            '/project/7/wiki/project/how-we-work',
            '/project/7/wiki/project/gateway',
            '/project/7/wiki/project/retention',
            '/project/7/wiki/instance/security'
        ]);
    });

    it('reloads when the run moves on or its stats arrive, but not for another run', () => {
        render(run());
        expect(loadWikiReads).toHaveBeenCalledTimes(1);

        stats.next(statsNotice(99));
        expect(loadWikiReads).toHaveBeenCalledTimes(1);

        stats.next(statsNotice(5));
        expect(loadWikiReads).toHaveBeenCalledTimes(2);

        render(run({ stages: [{ stage: 'design', status: 'awaiting_approval', attemptNo: 1 }] }));
        expect(loadWikiReads).toHaveBeenCalledTimes(3);
    });

    it('renders nothing for a run that read no wiki', () => {
        loadWikiReads.mockReturnValue(of([]));
        const el = render(run());

        expect(el.querySelector('[data-testid="run-wiki-context"]')).toBeNull();
    });
});
