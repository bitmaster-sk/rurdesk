import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRunApi } from '../../../agent/api/agent-run.api.service';
import { AgentRun } from '../../../agent/model/agent-run.model';
import { AgentStage } from '../../../agent/model/agent-stage.enum';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { ProjectSkillApi } from '../../../project/api/project-skill.api.service';
import { UiModule } from '../../../ui/ui.module';
import { SkillApi } from '../../api/skill.api.service';
import { AgentOverview } from '../../../agent/model/agent-overview.model';
import { ProjectSkill } from '../../../project/model/project-skill.model';
import { Skill } from '../../model/skill.model';
import { AgentStagePlanEditorComponent } from '../../../agent/components/agent-stage-plan-editor/agent-stage-plan-editor.component';
import { AgentStagePlanFormComponent } from '../../../agent/components/agent-stage-plan-form/agent-stage-plan-form.component';
import { AgentDockComponent, AgentDockKind } from './agent-dock.component';

const AGENT: User = Fixtures.agent();
const SKILLS: Skill[] = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'TDD' })
];
const DEFAULTS: ProjectSkill[] = [{ idProject: 7, idSkill: 1, stage: AgentStage.Implementation }];
const RUN: AgentRun = Fixtures.agentRun();

const WORKLOAD: AgentOverview = {
    idUserAgent: 8,
    isBusy: true,
    current: { idIssuePublic: 42, releaseName: null, stage: AgentStage.Design },
    queueCount: 3,
    queuedIdsIssuePublic: [43],
    completedToday: 2,
    tokens7d: 412_534,
    avgRunDurationMs7d: 480_000,
    failedAttempts7d: 1,
    models: [],
    modelDefault: null,
    maxTurnsPlan: 250,
    maxTurnsImplement: 500,
    maxTurnsLimit: null
};

const WITH_MODELS: AgentOverview = {
    ...WORKLOAD,
    models: ['openai/kimi', 'ollama/qwen3:8b'],
    modelDefault: 'openai/kimi'
};

abstract class Dom {
    public static chip(root: HTMLElement, idSkill: number, stage: AgentStage): HTMLButtonElement {
        return root.querySelector(`[data-testid="chip-${idSkill}-${stage}"]`) as HTMLButtonElement;
    }

    public static byTestId(root: HTMLElement, testId: string): HTMLElement {
        return root.querySelector(`[data-testid="${testId}"]`) as HTMLElement;
    }

    public static openStage(root: HTMLElement, stage: AgentStage): void {
        Dom.byTestId(root, `stage-plan-toggle-${stage}`).click();
    }

    public static async pick(
        fixture: ComponentFixture<AgentDockComponent>,
        testId: string,
        option: string
    ): Promise<void> {
        const root = fixture.nativeElement as HTMLElement;
        (Dom.byTestId(root, testId).querySelector('.ui-select-trigger') as HTMLElement).click();
        fixture.detectChanges();
        await fixture.whenStable();
        const match = Array.from(
            document.querySelectorAll<HTMLElement>('.ui-select-panel__option')
        ).find(el => el.textContent?.includes(option));
        match!.click();
        fixture.detectChanges();
    }

    public static runModel(root: HTMLElement): string {
        return Dom.text(root, 'agent-dock-run-model');
    }

    public static stageTurns(root: HTMLElement, stage: AgentStage): HTMLInputElement {
        return root.querySelector(`[data-testid="stage-turns-${stage}"]`) as HTMLInputElement;
    }

    public static text(root: HTMLElement, testId: string): string {
        return Dom.byTestId(root, testId)?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    }
}

describe('AgentDockComponent (browser)', () => {
    let skillApi: { load$: ReturnType<typeof vi.fn> };
    let projectSkillApi: { load$: ReturnType<typeof vi.fn> };
    let agentRunApi: {
        assignAgent$: ReturnType<typeof vi.fn>;
        loadByIssue$: ReturnType<typeof vi.fn>;
    };
    let toast: { showError: ReturnType<typeof vi.fn> };
    let closeSpy: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        skillApi = { load$: vi.fn().mockReturnValue(of(SKILLS)) };
        projectSkillApi = { load$: vi.fn().mockReturnValue(of(DEFAULTS)) };
        agentRunApi = {
            assignAgent$: vi.fn().mockReturnValue(of(RUN)),
            loadByIssue$: vi.fn().mockReturnValue(of(null))
        };
        toast = { showError: vi.fn() };
        closeSpy = vi.fn();
    });

    async function setup(kind: AgentDockKind, overview: AgentOverview | null = null) {
        await TestBed.configureTestingModule({
            imports: [UiModule, FormsModule, TranslateModule.forRoot(), TablerIconStub],
            declarations: [
                AgentDockComponent,
                AgentStagePlanEditorComponent,
                AgentStagePlanFormComponent
            ],
            providers: [
                { provide: SkillApi, useValue: skillApi },
                { provide: ProjectSkillApi, useValue: projectSkillApi },
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: ToastNotificationService, useValue: toast }
            ]
        }).compileComponents();

        const fixture = TestBed.createComponent(AgentDockComponent);
        fixture.componentRef.setInput('agent', AGENT);
        fixture.componentRef.setInput('kind', kind);
        fixture.componentRef.setInput('idProject', 7);
        fixture.componentRef.setInput('idIssuePublic', 42);
        fixture.componentRef.setInput('overview', overview);
        fixture.componentRef.setInput('close', closeSpy);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    function openAllStages(fixture: ComponentFixture<AgentDockComponent>): void {
        const root = fixture.nativeElement as HTMLElement;
        [
            AgentStage.Brainstorming,
            AgentStage.Design,
            AgentStage.ImplementationPlan,
            AgentStage.Implementation
        ].forEach(stage => Dom.openStage(root, stage));
        fixture.detectChanges();
    }

    it('renders a chip per skill and stage, pre-checked from the project matrix', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;
        openAllStages(fixture);

        expect(root.querySelectorAll('[data-testid^="chip-"]').length).toBe(8);
        expect(Dom.chip(root, 1, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'true'
        );
        expect(Dom.chip(root, 2, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'false'
        );
        expect(Dom.chip(root, 1, AgentStage.Design).getAttribute('aria-pressed')).toBe('false');
    });

    it('assigns with exactly the chips the user left checked', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;
        const assigned = vi.fn();
        fixture.componentInstance.assigned.subscribe(assigned);
        openAllStages(fixture);

        Dom.chip(root, 2, AgentStage.Design).click();
        fixture.detectChanges();
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(7, 42, {
            idUserAgent: 8,
            idsSkillByStage: { implementation: [1], design: [2] },
            isShortWorkflow: false
        });
        expect(assigned).toHaveBeenCalledWith(RUN);
        expect(closeSpy).toHaveBeenCalled();
    });

    it('a paused run prefills its own skills and workflow, not the project defaults', async () => {
        agentRunApi.loadByIssue$ = vi.fn().mockReturnValue(
            of({
                ...RUN,
                isParked: true,
                stagePlan: {
                    stages: [
                        { name: AgentStage.Design, skippable: true, skip: true, idsSkill: [] },
                        {
                            name: AgentStage.ImplementationPlan,
                            skippable: false,
                            skip: true,
                            idsSkill: []
                        },
                        {
                            name: AgentStage.Implementation,
                            skippable: false,
                            skip: false,
                            idsSkill: [2]
                        }
                    ]
                }
            })
        );
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;
        Dom.openStage(root, AgentStage.Implementation);
        fixture.detectChanges();

        expect(Dom.chip(root, 2, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'true'
        );
        expect(Dom.chip(root, 1, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'false'
        );
        expect(projectSkillApi.load$).not.toHaveBeenCalled();
        expect(Dom.text(root, 'agent-dock-prefilled')).toBe('AGENT_DOCK.PREFILLED_FROM_RUN');

        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(7, 42, {
            idUserAgent: 8,
            idsSkillByStage: { implementation: [2] },
            isShortWorkflow: true
        });
    });

    it('a run that is not paused keeps the project defaults', async () => {
        agentRunApi.loadByIssue$ = vi.fn().mockReturnValue(of({ ...RUN, isParked: false }));
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.byTestId(root, 'agent-dock-prefilled')).toBeNull();
        Dom.openStage(root, AgentStage.Implementation);
        fixture.detectChanges();

        expect(Dom.chip(root, 1, AgentStage.Implementation).getAttribute('aria-pressed')).toBe(
            'true'
        );
    });

    it('assigns with the short workflow when the user turns it on', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;

        Dom.byTestId(root, 'agent-dock-short-workflow').click();
        fixture.detectChanges();
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(
            7,
            42,
            expect.objectContaining({ isShortWorkflow: true })
        );
    });

    it('a failed assign toasts and does not emit', async () => {
        agentRunApi.assignAgent$ = vi.fn().mockReturnValue(throwError(() => new Error('nope')));
        const fixture = await setup(AgentDockKind.Skills);
        const assigned = vi.fn();
        fixture.componentInstance.assigned.subscribe(assigned);

        Dom.byTestId(fixture.nativeElement, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(toast.showError).toHaveBeenCalledWith('AGENT_DOCK.ASSIGN_ERROR');
        expect(assigned).not.toHaveBeenCalled();
        expect(closeSpy).not.toHaveBeenCalled();
    });

    it('a refused assign leaves the specific server error to the interceptor toast', async () => {
        agentRunApi.assignAgent$ = vi.fn().mockReturnValue(
            throwError(() => ({
                status: 409,
                error: { code: 'WORKFLOW_LOCKED', translateKey: 'error.workflow_locked' }
            }))
        );
        const fixture = await setup(AgentDockKind.Skills);
        const assigned = vi.fn();
        fixture.componentInstance.assigned.subscribe(assigned);

        Dom.byTestId(fixture.nativeElement, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(toast.showError).not.toHaveBeenCalledWith('AGENT_DOCK.ASSIGN_ERROR');
        expect(assigned).not.toHaveBeenCalled();
        expect(closeSpy).not.toHaveBeenCalled();
    });

    it('offers the models the agent reported, with its default picked', async () => {
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.runModel(root)).toContain('openai/kimi');
        expect(Dom.runModel(root)).toContain('AGENT_DOCK.MODEL_DEFAULT');

        (
            root.querySelector(
                '[data-testid="agent-dock-run-model"] .ui-select-trigger'
            ) as HTMLElement
        ).click();
        fixture.detectChanges();
        const offered = Array.from(document.querySelectorAll('.ui-select-panel__option'));
        expect(offered.map(el => el.textContent?.includes('AGENT_DOCK.MODEL_DEFAULT'))).toEqual([
            true,
            false
        ]);
        expect(offered[1].textContent).toContain('ollama/qwen3:8b');
    });

    it('assigns with the model the user picked for the run', async () => {
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        await Dom.pick(fixture, 'agent-dock-run-model', 'ollama/qwen3:8b');
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(
            7,
            42,
            expect.objectContaining({ model: 'ollama/qwen3:8b' })
        );
    });

    it('shows no model choice for an agent that reported no models', async () => {
        const fixture = await setup(AgentDockKind.Skills, WORKLOAD);

        expect(Dom.byTestId(fixture.nativeElement, 'agent-dock-model')).toBeNull();
    });

    it('an opened stage sends its own model and turn limit', async () => {
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.stageTurns(root, AgentStage.Design)).toBeNull();
        openAllStages(fixture);

        expect(Dom.stageTurns(root, AgentStage.Design).placeholder).toBe('250');
        expect(Dom.stageTurns(root, AgentStage.Implementation).placeholder).toBe('500');
        await Dom.pick(fixture, `stage-model-${AgentStage.Design}`, 'ollama/qwen3:8b');
        const turns = Dom.stageTurns(root, AgentStage.Implementation);
        turns.value = '700';
        turns.dispatchEvent(new Event('change'));
        fixture.detectChanges();
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(
            7,
            42,
            expect.objectContaining({
                modelByStage: { design: 'ollama/qwen3:8b' },
                maxTurnsByStage: { implementation: 700 }
            })
        );
    });

    it('a paused run prefills its model choices', async () => {
        agentRunApi.loadByIssue$ = vi.fn().mockReturnValue(
            of({
                ...RUN,
                isParked: true,
                stagePlan: {
                    model: 'ollama/qwen3:8b',
                    stages: [
                        {
                            name: AgentStage.Design,
                            skippable: true,
                            skip: false,
                            model: 'openai/kimi',
                            maxTurns: 40
                        }
                    ]
                }
            })
        );
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        expect(agentRunApi.assignAgent$).toHaveBeenCalledWith(
            7,
            42,
            expect.objectContaining({
                model: 'ollama/qwen3:8b',
                modelByStage: { design: 'openai/kimi' },
                maxTurnsByStage: { design: 40 }
            })
        );
    });

    it('a paused run drops the models this agent does not offer', async () => {
        agentRunApi.loadByIssue$ = vi.fn().mockReturnValue(
            of({
                ...RUN,
                isParked: true,
                stagePlan: {
                    model: 'anthropic/opus',
                    stages: [
                        {
                            name: AgentStage.Design,
                            skippable: true,
                            skip: false,
                            model: 'anthropic/opus'
                        },
                        {
                            name: AgentStage.Implementation,
                            skippable: false,
                            skip: false,
                            model: 'openai/kimi'
                        }
                    ]
                }
            })
        );
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.runModel(root)).toContain('openai/kimi');
        Dom.byTestId(root, 'agent-dock-assign').click();
        fixture.detectChanges();

        const body = agentRunApi.assignAgent$.mock.calls[0][2];
        expect(body.model).toBeUndefined();
        expect(body.modelByStage).toEqual({ implementation: 'openai/kimi' });
    });

    it('a closed stage summarises its skills, model and turns', async () => {
        const fixture = await setup(AgentDockKind.Skills, WITH_MODELS);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.chip(root, 1, AgentStage.Implementation)).toBeNull();
        expect(Dom.text(root, `stage-plan-summary-skills-${AgentStage.Implementation}`)).toBe(
            'Verification rules'
        );
        expect(Dom.text(root, `stage-plan-summary-skills-${AgentStage.Design}`)).toBe(
            'AGENT_STAGE_PLAN_EDITOR.NONE'
        );
        expect(Dom.text(root, `stage-plan-summary-model-${AgentStage.Implementation}`)).toBe(
            'AGENT_STAGE_PLAN_EDITOR.MODEL_INHERIT · AGENT_STAGE_PLAN_EDITOR.TURNS'
        );

        Dom.openStage(root, AgentStage.Design);
        fixture.detectChanges();
        await Dom.pick(fixture, `stage-model-${AgentStage.Design}`, 'ollama/qwen3:8b');
        Dom.openStage(root, AgentStage.Design);
        fixture.detectChanges();

        expect(Dom.text(root, `stage-plan-summary-model-${AgentStage.Design}`)).toContain(
            'ollama/qwen3:8b'
        );
    });

    it('the short workflow marks design and implementation plan as skipped', async () => {
        const fixture = await setup(AgentDockKind.Skills);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.byTestId(root, `stage-plan-locked-${AgentStage.Design}`)).toBeNull();
        Dom.byTestId(root, 'agent-dock-short-workflow').click();
        fixture.detectChanges();

        expect(Dom.byTestId(root, `stage-plan-locked-${AgentStage.Design}`)).not.toBeNull();
        expect(
            Dom.byTestId(root, `stage-plan-locked-${AgentStage.ImplementationPlan}`)
        ).not.toBeNull();
        expect(Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Design}`)).toBeNull();
        expect(Dom.byTestId(root, `stage-plan-toggle-${AgentStage.Implementation}`)).not.toBeNull();
    });

    it('info mode renders the workload and names this project queued tasks', async () => {
        const fixture = await setup(AgentDockKind.Info, WORKLOAD);
        const root = fixture.nativeElement as HTMLElement;

        expect(Dom.text(root, 'agent-dock-running')).toContain('#42');
        expect(Dom.text(root, 'agent-dock-queue')).toContain('3');
        expect(Dom.text(root, 'agent-dock-queue')).toContain('#43');
        expect(Dom.text(root, 'agent-dock-tokens')).toBe('412.5k');
        expect(skillApi.load$).not.toHaveBeenCalled();
    });

    it('info mode says so when the agent is running nothing here', async () => {
        const fixture = await setup(AgentDockKind.Info, {
            ...WORKLOAD,
            current: null,
            queuedIdsIssuePublic: []
        });

        expect(Dom.text(fixture.nativeElement, 'agent-dock-running')).toBe('AGENT_DOCK.NONE');
    });
});
