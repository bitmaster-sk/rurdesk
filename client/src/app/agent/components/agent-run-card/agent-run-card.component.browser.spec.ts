import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UiModule } from '../../../ui/ui.module';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { AgentPhase } from '../../model/agent-phase.enum';
import { AgentRun, AgentStageProgress, RunStats } from '../../model/agent-run.model';
import { AgentRunStagePlanRes } from '../../model/agent-run-stage-plan.model';
import { AgentRunWikiRead } from '../../model/agent-run-wiki-read.model';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentWikiReadSource } from '../../model/agent-wiki-read-source.enum';
import { AgentRunSummaryComponent } from '../agent-run-summary/agent-run-summary.component';
import { AgentStageCardComponent } from '../agent-stage-card/agent-stage-card.component';
import { AgentStagePlanFormComponent } from '../agent-stage-plan-form/agent-stage-plan-form.component';
import { RunWikiStageComponent } from '../run-wiki-stage/run-wiki-stage.component';
import { AgentRunCardComponent } from './agent-run-card.component';

@Component({ selector: 'app-run-recovery-banner', template: '', standalone: true })
class RunRecoveryBannerStub {
    public readonly idRun = input<number>(0);
    public readonly errorKey = input<string | null>(null);
    public readonly errorDetail = input<string | null>(null);
    public readonly continued = output<void>();
    public readonly restarted = output<void>();
}

const STATS: RunStats = {
    totalTokensUsed: 206493,
    totalDurationMs: 99000,
    totalToolCallsCount: 26,
    attemptsPerStage: {},
    failedAttempts: 0,
    tokensByModel: { 'openai/kimi': 206493 }
};

const STAGE_PLAN: AgentRunStagePlanRes = {
    stages: [AgentStage.Brainstorming, AgentStage.Design, AgentStage.Implementation].map(name => ({
        name,
        idsSkill: [],
        model: null,
        maxTurns: null,
        skip: false,
        dispatched: name !== AgentStage.Implementation
    })),
    models: ['openai/kimi'],
    modelDefault: 'openai/kimi',
    maxTurnsPlan: 250,
    maxTurnsImplement: 500,
    maxTurnsLimit: null
};

function designWikiRead(): AgentRunWikiRead {
    return {
        idRead: 1,
        idRun: 1,
        idTask: 50,
        stage: AgentStage.Design,
        idCall: 'prompt',
        source: AgentWikiReadSource.PromptAlways,
        idPage: 10,
        spaceKind: WikiSpaceKind.Project,
        slug: 'commit-messages',
        title: 'Commit messages',
        versionNo: 1,
        tokens: 40,
        query: null,
        pages: null,
        createAt: '2026-10-20T10:00:00Z'
    };
}

describe('AgentRunCardComponent', () => {
    let fixture: ComponentFixture<AgentRunCardComponent>;
    let agentRunApi: Record<string, ReturnType<typeof vi.fn>>;

    beforeEach(async () => {
        agentRunApi = {
            loadStats$: vi.fn().mockReturnValue(of(STATS)),
            loadWikiReads$: vi.fn().mockReturnValue(of([designWikiRead()])),
            loadStagePlan$: vi.fn().mockReturnValue(of(STAGE_PLAN)),
            updateStagePlanEntry$: vi.fn().mockReturnValue(of(STAGE_PLAN)),
            updateWorkflow$: vi.fn()
        };
        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                UiModule,
                FormsModule,
                TablerIconStub,
                RouterModule,
                RunRecoveryBannerStub
            ],
            declarations: [
                AgentRunCardComponent,
                AgentRunSummaryComponent,
                AgentStageCardComponent,
                AgentStagePlanFormComponent,
                RunWikiStageComponent
            ],
            providers: [
                provideRouter([]),
                { provide: AgentRunApi, useValue: agentRunApi },
                {
                    provide: SkillApi,
                    useValue: {
                        load$: vi
                            .fn()
                            .mockReturnValue(of([Fixtures.skill({ idSkill: 1, name: 'TDD' })]))
                    }
                },
                { provide: ToastNotificationService, useValue: { showError: vi.fn() } },
                { provide: NoticeService, useValue: { agentStats$: new Subject() } }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(AgentRunCardComponent);
    });

    function renderWithPhase(phase: AgentPhase): HTMLElement {
        fixture.componentRef.setInput('run', {
            idRun: 1,
            phase,
            prUrl: 'https://github.com/acme/repo/pull/7',
            stages: []
        });
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    it('does not pulse the avatar once a PR is open (agent has finished)', () => {
        const el = renderWithPhase(AgentPhase.PrOpen);
        expect(el.querySelector('.agent-run-card.is-active')).toBeNull();
    });

    it('pulses the avatar while the agent is working', () => {
        const el = renderWithPhase(AgentPhase.InProgress);
        expect(el.querySelector('.agent-run-card.is-active')).not.toBeNull();
    });

    it('hides the cancel action in pr_open — the work is done, merge happens on the host', () => {
        const el = renderWithPhase(AgentPhase.PrOpen);
        expect(el.querySelectorAll('.summary-actions button').length).toBe(0);
    });

    it('offers a cancel action while the run is still in progress', () => {
        const el = renderWithPhase(AgentPhase.InProgress);
        expect(el.querySelectorAll('.summary-actions button').length).toBe(1);
    });

    describe('parked run', () => {
        function renderParked(idAssignee: number | null): HTMLElement {
            fixture.componentRef.setInput('usersMap', new Map([[5, { idUser: 5, name: 'Jana' }]]));
            fixture.componentRef.setInput('idAssignee', idAssignee);
            fixture.componentRef.setInput('run', {
                idRun: 1,
                phase: AgentPhase.InProgress,
                isParked: true,
                stages: []
            });
            fixture.detectChanges();
            return fixture.nativeElement as HTMLElement;
        }

        it('does not pulse or look like it is working', () => {
            const el = renderParked(null);
            expect(el.querySelector('.agent-run-card.is-active')).toBeNull();
            expect(el.querySelector('.agent-run-card.is-working')).toBeNull();
            expect(el.querySelector('.card-summary')?.textContent).toContain('AGENT.PHASE.PAUSED');
        });

        async function hoverPhaseTag(el: HTMLElement): Promise<string> {
            el.querySelector('[data-testid="run-phase-tag"]')!.dispatchEvent(
                new MouseEvent('mouseenter')
            );
            await new Promise(resolve => setTimeout(resolve, 220));
            return document.querySelector('.ui-tooltip')?.textContent ?? '';
        }

        it('explains in the tag tooltip that the task has no agent', async () => {
            const el = renderParked(null);
            expect(await hoverPhaseTag(el)).toContain('AGENT.CARD.PARKED_UNASSIGNED');
        });

        it('explains in the tag tooltip that the task went to someone else', async () => {
            const el = renderParked(5);
            expect(await hoverPhaseTag(el)).toContain('AGENT.CARD.PARKED_REASSIGNED');
        });

        it('keeps the panel free of a parked notice', () => {
            const el = renderParked(null);
            expect(el.querySelector('.card-body')).toBeNull();
        });

        it('offers to assign the agent back', () => {
            const el = renderParked(null);
            const assignBack = vi.fn();
            fixture.componentInstance.assignBackRun.subscribe(assignBack);

            (
                el.querySelector('[data-testid="run-assign-back"] button') as HTMLButtonElement
            ).click();

            expect(assignBack).toHaveBeenCalled();
        });
    });

    describe('stages', () => {
        function renderOpen(
            stages: AgentStageProgress[],
            overrides: Partial<AgentRun> = {}
        ): HTMLElement {
            fixture.componentRef.setInput(
                'run',
                Fixtures.agentRun({
                    idRun: 1,
                    phase: AgentPhase.InProgress,
                    stagePlan: {
                        model: 'openai/kimi',
                        stages: stages.map(stage => ({
                            name: stage.stage,
                            skippable: true,
                            skip: stage.status === 'skipped'
                        }))
                    },
                    stages,
                    ...overrides
                })
            );
            fixture.detectChanges();
            const el = fixture.nativeElement as HTMLElement;
            (el.querySelector('.card-summary') as HTMLButtonElement).click();
            fixture.detectChanges();
            return el;
        }

        function byTestId(el: HTMLElement, testId: string): HTMLElement | null {
            return el.querySelector(`[data-testid="${testId}"]`);
        }

        function openStage(el: HTMLElement, stage: string): void {
            byTestId(el, `stage-card-toggle-${stage}`)!.click();
            fixture.detectChanges();
        }

        const RAN = [
            Fixtures.stageProgress({
                stage: AgentStage.Brainstorming,
                status: 'done',
                attempts: 1,
                model: 'ollama/qwen3:8b'
            }),
            Fixtures.stageProgress({
                stage: AgentStage.Design,
                status: 'awaiting_approval',
                attempts: 1,
                model: 'openai/kimi'
            }),
            Fixtures.stageProgress({ stage: AgentStage.Implementation })
        ];

        it('puts everything about a stage in its card instead of separate sections', () => {
            const el = renderOpen(RAN);

            expect(el.textContent).not.toContain('AGENT.SIDEBAR.TIMELINE');
            expect(el.textContent).not.toContain('AGENT.STATS.TITLE');
            expect(byTestId(el, 'run-stage-plan-edit')).toBeNull();
            expect(
                Array.from(el.querySelectorAll('app-agent-stage-card')).map(card =>
                    card.querySelector('section')?.getAttribute('data-testid')
                )
            ).toEqual([
                'stage-card-brainstorming',
                'stage-card-design',
                'stage-card-implementation'
            ]);
        });

        it('shows the run model, workflow and totals above the stages', () => {
            const el = renderOpen(RAN);

            expect(byTestId(el, 'run-model')?.textContent).toContain('openai/kimi');
            expect(byTestId(el, 'run-tokens')?.textContent).toContain('206,493');
            expect(byTestId(el, 'run-tool-calls')?.textContent).toContain('26');
        });

        it('says the run uses the gateway default when no model was chosen', () => {
            const el = renderOpen(RAN, { stagePlan: { stages: [] } });

            expect(byTestId(el, 'run-model')?.textContent).toContain(
                'AGENT.SIDEBAR.MODEL_GATEWAY_DEFAULT'
            );
        });

        it('shows the model each stage ran with once the stage is opened', () => {
            const el = renderOpen(RAN);
            openStage(el, AgentStage.Brainstorming);

            expect(byTestId(el, 'stage-card-ran-with-brainstorming')?.textContent).toContain(
                'ollama/qwen3:8b'
            );
        });

        it('warns when a stage fell back to the gateway default model', () => {
            const el = renderOpen([
                Fixtures.stageProgress({
                    stage: AgentStage.Design,
                    status: 'awaiting_approval',
                    attempts: 1,
                    model: 'openai/kimi',
                    modelFallbackFrom: 'anthropic/opus'
                })
            ]);

            expect(byTestId(el, 'stage-card-model-fallback-design')?.textContent).toContain(
                'AGENT.TIMELINE.MODEL_FALLBACK'
            );
        });

        it('shows the wiki of a stage only in that stage', () => {
            const el = renderOpen(RAN);
            openStage(el, AgentStage.Brainstorming);

            expect(byTestId(el, 'stage-card-body-brainstorming')?.textContent).not.toContain(
                'Commit messages'
            );
            expect(byTestId(el, 'stage-card-body-design')?.textContent).toContain(
                'Commit messages'
            );
        });

        it('lets the user edit the next stage in its card and saves the change', () => {
            const el = renderOpen(RAN);

            expect(byTestId(el, 'run-stages-not-started')).not.toBeNull();
            byTestId(el, 'chip-1-implementation')!.click();

            expect(agentRunApi['updateStagePlanEntry$']).toHaveBeenCalledWith(
                1,
                expect.objectContaining({ name: 'implementation', idsSkill: [1] })
            );
        });

        it('turns the editor into a read-only card once the stage has started', () => {
            const el = renderOpen(RAN);
            fixture.componentRef.setInput(
                'run',
                Fixtures.agentRun({
                    idRun: 1,
                    phase: AgentPhase.InProgress,
                    stages: [
                        ...RAN.slice(0, 2),
                        Fixtures.stageProgress({
                            stage: AgentStage.Implementation,
                            status: 'active',
                            attempts: 1
                        })
                    ]
                })
            );
            fixture.detectChanges();
            openStage(el, AgentStage.Implementation);

            expect(byTestId(el, 'stage-plan-form-implementation')).toBeNull();
            expect(byTestId(el, 'stage-card-usage-implementation')).not.toBeNull();
        });

        it('offers no editor once the run has finished', () => {
            const el = renderOpen(RAN, { phase: AgentPhase.Done });

            expect(byTestId(el, 'stage-plan-form-implementation')).toBeNull();
            expect(byTestId(el, 'run-stages-not-started')).toBeNull();
        });

        it('shows a skipped stage without an editor', () => {
            const el = renderOpen([
                ...RAN.slice(0, 1),
                Fixtures.stageProgress({ stage: AgentStage.Design, status: 'skipped' }),
                RAN[2]
            ]);

            expect((byTestId(el, 'stage-card-toggle-design') as HTMLButtonElement).disabled).toBe(
                true
            );
            expect(byTestId(el, 'stage-plan-form-design')).toBeNull();
        });

        it('offers the short workflow toggle while the workflow can still change', () => {
            const el = renderOpen([
                Fixtures.stageProgress({ stage: AgentStage.Brainstorming, status: 'active' })
            ]);

            expect(byTestId(el, 'run-short-workflow')).not.toBeNull();
            expect(byTestId(el, 'run-workflow-locked')).toBeNull();
        });

        it('shows a locked workflow as a value once design has started', () => {
            const el = renderOpen([
                Fixtures.stageProgress({ stage: AgentStage.Design, status: 'active', attempts: 1 })
            ]);

            expect(byTestId(el, 'run-short-workflow')).toBeNull();
            expect(byTestId(el, 'run-workflow')?.textContent).toContain(
                'AGENT.WORKFLOW.FULL_VALUE'
            );
            expect(byTestId(el, 'run-workflow-locked')).not.toBeNull();
        });
    });
});
