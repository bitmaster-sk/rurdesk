import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';

import { AgentRunCardComponent } from './agent-run-card.component';
import { AgentPhase } from '../../model/agent-phase.enum';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { vi } from 'vitest';

@Component({ selector: 'app-run-recovery-banner', template: '', standalone: true })
class RunRecoveryBannerStub {
    public readonly idRun = input<number>(0);
    public readonly errorKey = input<string | null>(null);
    public readonly errorDetail = input<string | null>(null);
    public readonly continued = output<void>();
    public readonly restarted = output<void>();
}

@Component({ selector: 'app-run-wiki-context', template: '', standalone: true })
class RunWikiContextStub {
    public readonly run = input<unknown>(null);
}

@Component({ selector: 'app-agent-stage-plan-editor', template: '', standalone: true })
class AgentStagePlanEditorStub {
    public readonly rows = input<unknown[]>([]);
    public readonly skills = input<unknown[]>([]);
    public readonly models = input<string[]>([]);
    public readonly maxTurnsLimit = input<number | null>(null);
    public readonly saveStatusByStage = input<Record<string, string>>({});
    public readonly skillToggled = output<unknown>();
    public readonly modelPicked = output<unknown>();
    public readonly maxTurnsChanged = output<unknown>();
}

@Component({ selector: 'app-run-stats-panel', template: '', standalone: true })
class RunStatsPanelStub {
    public readonly idRun = input<number>(0);
}

describe('AgentRunCardComponent', () => {
    let fixture: ComponentFixture<AgentRunCardComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                UiModule,
                TablerIconStub,
                RunRecoveryBannerStub,
                RunStatsPanelStub,
                RunWikiContextStub,
                AgentStagePlanEditorStub
            ],
            declarations: [AgentRunCardComponent]
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

    describe('timeline', () => {
        function renderOpenWithStages(stages: unknown[], stagePlan: unknown = null): HTMLElement {
            fixture.componentRef.setInput('run', {
                idRun: 1,
                phase: AgentPhase.InProgress,
                prUrl: null,
                stagePlan,
                stages
            });
            fixture.detectChanges();
            const el = fixture.nativeElement as HTMLElement;
            (el.querySelector('.card-summary') as HTMLButtonElement).click();
            fixture.detectChanges();
            return el;
        }

        it('shows the model each stage ran with', () => {
            const el = renderOpenWithStages([
                { stage: 'design', status: 'done', model: 'ollama/qwen3:8b' },
                { stage: 'implementation', status: 'pending' }
            ]);

            expect(
                el.querySelector('[data-testid="timeline-model-design"]')?.textContent
            ).toContain('ollama/qwen3:8b');
            expect(el.querySelector('[data-testid="timeline-model-implementation"]')).toBeNull();
        });

        it('warns when a stage fell back to the gateway default model', () => {
            const el = renderOpenWithStages([
                {
                    stage: 'design',
                    status: 'active',
                    model: 'openai/kimi',
                    modelFallbackFrom: 'anthropic/opus'
                },
                { stage: 'implementation', status: 'done', model: 'openai/kimi' }
            ]);

            expect(
                el.querySelector('[data-testid="timeline-model-fallback-design"]')?.textContent
            ).toContain('AGENT.TIMELINE.MODEL_FALLBACK');
            expect(
                el.querySelector('[data-testid="timeline-model-fallback-implementation"]')
            ).toBeNull();
        });
        it('shows the model chosen for the run in the header', () => {
            const el = renderOpenWithStages([], { model: 'openai/glm-5.3', stages: [] });

            expect(el.querySelector('[data-testid="run-model"]')?.textContent).toContain(
                'openai/glm-5.3'
            );
        });

        it('says the run uses the gateway default when no model was chosen', () => {
            const el = renderOpenWithStages([], { stages: [] });

            expect(el.querySelector('[data-testid="run-model"]')?.textContent).toContain(
                'AGENT.SIDEBAR.MODEL_GATEWAY_DEFAULT'
            );
        });

        it('offers the short workflow toggle while the workflow can still change', () => {
            const el = renderOpenWithStages([{ stage: 'brainstorming', status: 'running' }]);

            expect(el.querySelector('[data-testid="run-short-workflow"]')).not.toBeNull();
            expect(el.querySelector('[data-testid="run-workflow-locked"]')).toBeNull();
        });

        it('shows a locked workflow as a value once design has started', () => {
            const el = renderOpenWithStages([{ stage: 'design', status: 'running' }]);

            expect(el.querySelector('[data-testid="run-short-workflow"]')).toBeNull();
            expect(el.querySelector('[data-testid="run-workflow"]')?.textContent).toContain(
                'AGENT.WORKFLOW.FULL_VALUE'
            );
            expect(el.querySelector('[data-testid="run-workflow-locked"]')).not.toBeNull();
        });

        it('offers to edit the stage plan while the run is going', () => {
            const el = renderOpenWithStages([], { stages: [] });

            expect(el.querySelector('[data-testid="run-stage-plan-edit"]')).not.toBeNull();
        });

        it('a finished run has no stage plan to edit', () => {
            fixture.componentRef.setInput('run', {
                idRun: 1,
                phase: AgentPhase.Done,
                prUrl: null,
                stagePlan: { stages: [] },
                stages: []
            });
            fixture.detectChanges();
            const el = fixture.nativeElement as HTMLElement;
            (el.querySelector('.card-summary') as HTMLButtonElement).click();
            fixture.detectChanges();

            expect(el.querySelector('[data-testid="run-stage-plan-edit"]')).toBeNull();
        });

        it('never plans a model for pickup, which runs without one', () => {
            const el = renderOpenWithStages(
                [
                    { stage: 'pickup', status: 'pending' },
                    { stage: 'design', status: 'pending' }
                ],
                {
                    model: 'openai/glm-5.3',
                    stages: [
                        { name: 'pickup', skippable: false, skip: false },
                        { name: 'design', skippable: true, skip: false }
                    ]
                }
            );

            expect(el.querySelector('[data-testid="timeline-model-pickup"]')).toBeNull();
            expect(
                el.querySelector('[data-testid="timeline-model-design"]')?.textContent
            ).toContain('openai/glm-5.3');
        });

        it('shows the planned model of a stage that has not started yet', () => {
            const el = renderOpenWithStages(
                [
                    { stage: 'design', status: 'pending' },
                    { stage: 'implementation', status: 'pending' },
                    { stage: 'brainstorming', status: 'done', model: 'openai/kimi-k2.7-code' }
                ],
                {
                    model: 'openai/glm-5.3',
                    stages: [
                        {
                            name: 'design',
                            skippable: true,
                            skip: false,
                            model: 'openai/mistral-large-4'
                        },
                        { name: 'implementation', skippable: false, skip: false },
                        { name: 'brainstorming', skippable: false, skip: false }
                    ]
                }
            );

            const design = el.querySelector('[data-testid="timeline-model-design"]')?.textContent;
            expect(design).toContain('openai/mistral-large-4');
            expect(design).toContain('AGENT.TIMELINE.MODEL_PLANNED');
            expect(
                el.querySelector('[data-testid="timeline-model-implementation"]')?.textContent
            ).toContain('openai/glm-5.3');
            const brainstorming = el.querySelector(
                '[data-testid="timeline-model-brainstorming"]'
            )?.textContent;
            expect(brainstorming).toContain('openai/kimi-k2.7-code');
            expect(brainstorming).not.toContain('AGENT.TIMELINE.MODEL_PLANNED');
        });
    });
});
