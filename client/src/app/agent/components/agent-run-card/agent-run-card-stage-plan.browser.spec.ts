import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Fixtures } from 'src/testing/fixtures';
import { AgentRunApi } from '../../api/agent-run.api.service';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';
import { AgentRunStagePlanRes } from '../../model/agent-run-stage-plan.model';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentRunCardComponent } from './agent-run-card.component';

@Component({ selector: 'app-agent-stage-plan-editor', template: '', standalone: true })
class AgentStagePlanEditorStub {
    public readonly rows = input<AgentStagePlanEditorRow[]>([]);
    public readonly skills = input<Skill[]>([]);
    public readonly saveStatusByStage = input<Record<string, UiSaveState>>({});
    public readonly skillToggled = output<AgentStagePlanSkillToggle>();
    public readonly modelPicked = output<AgentStagePlanModelPick>();
    public readonly maxTurnsChanged = output<AgentStagePlanMaxTurnsChange>();
}

const CATALOG: Skill[] = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'TDD' })
];

function stagePlan(implementationSkills: number[]): AgentRunStagePlanRes {
    return {
        stages: [
            {
                name: AgentStage.Design,
                idsSkill: [1],
                model: null,
                maxTurns: null,
                skip: false,
                dispatched: true
            },
            {
                name: AgentStage.ImplementationPlan,
                idsSkill: [],
                model: null,
                maxTurns: null,
                skip: true,
                dispatched: false
            },
            {
                name: AgentStage.Implementation,
                idsSkill: implementationSkills,
                model: null,
                maxTurns: null,
                skip: false,
                dispatched: false
            }
        ],
        models: ['openai/a', 'openai/b'],
        modelDefault: 'openai/a',
        maxTurnsPlan: 250,
        maxTurnsImplement: 500,
        maxTurnsLimit: 700
    };
}

describe('AgentRunCardComponent — run stage plan (browser)', () => {
    let agentRunApi: {
        loadStagePlan$: ReturnType<typeof vi.fn>;
        updateStagePlanEntry$: ReturnType<typeof vi.fn>;
        updateWorkflow$: ReturnType<typeof vi.fn>;
    };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        agentRunApi = {
            loadStagePlan$: vi.fn().mockReturnValue(of(stagePlan([]))),
            updateStagePlanEntry$: vi.fn().mockReturnValue(of(stagePlan([2]))),
            updateWorkflow$: vi.fn().mockReturnValue(of(Fixtures.agentRun()))
        };
        toast = { showError: vi.fn() };
    });

    async function setup(): Promise<{
        fixture: ComponentFixture<AgentRunCardComponent>;
        editor: () => AgentStagePlanEditorStub;
    }> {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), AgentStagePlanEditorStub],
            declarations: [AgentRunCardComponent],
            providers: [
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: SkillApi, useValue: { load$: vi.fn().mockReturnValue(of(CATALOG)) } },
                { provide: ToastNotificationService, useValue: toast }
            ]
        })
            .overrideComponent(AgentRunCardComponent, {
                set: {
                    template: `
                        <button class="open" (click)="onOpenStagePlan()"></button>
                        <button class="workflow" (click)="onToggleShortWorkflow($event)"></button>
                        <app-agent-stage-plan-editor
                            [rows]="stagePlanRows()"
                            [skills]="skillCatalog()"
                            [saveStatusByStage]="stageStatus()"
                            (skillToggled)="onToggleSkill($event)"
                            (modelPicked)="onPickStageModel($event)"
                            (maxTurnsChanged)="onStageMaxTurns($event)"
                        />
                    `
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AgentRunCardComponent);
        fixture.componentRef.setInput('run', Fixtures.agentRun());
        fixture.detectChanges();
        fixture.nativeElement.querySelector('.open').click();
        fixture.detectChanges();
        const editor = () =>
            fixture.debugElement.query(By.directive(AgentStagePlanEditorStub))
                .componentInstance as AgentStagePlanEditorStub;
        return { fixture, editor };
    }

    it('locks a stage that already started or is skipped, and offers the default turns', async () => {
        const { editor } = await setup();

        expect(editor().rows()).toEqual([
            {
                stage: AgentStage.Design,
                idsSkill: [1],
                model: null,
                maxTurns: null,
                defaultMaxTurns: 250,
                lock: AgentStagePlanEditorLock.Started
            },
            {
                stage: AgentStage.ImplementationPlan,
                idsSkill: [],
                model: null,
                maxTurns: null,
                defaultMaxTurns: 250,
                lock: AgentStagePlanEditorLock.Skipped
            },
            {
                stage: AgentStage.Implementation,
                idsSkill: [],
                model: null,
                maxTurns: null,
                defaultMaxTurns: 500,
                lock: null
            }
        ]);
    });

    it('toggling a skill saves that stage and marks it saved', async () => {
        const { fixture, editor } = await setup();

        editor().skillToggled.emit({ stage: AgentStage.Implementation, idSkill: 2 });
        fixture.detectChanges();

        expect(agentRunApi.updateStagePlanEntry$).toHaveBeenCalledWith(
            55,
            expect.objectContaining({ name: 'implementation', idsSkill: [2] })
        );
        expect(editor().rows()[2].idsSkill).toEqual([2]);
        expect(editor().saveStatusByStage()['implementation']).toBe(UiSaveState.Saved);
    });

    it('picking a model keeps the stage skills and saves the model', async () => {
        const { editor } = await setup();

        editor().modelPicked.emit({ stage: AgentStage.Implementation, model: 'openai/b' });

        expect(agentRunApi.updateStagePlanEntry$).toHaveBeenCalledWith(55, {
            name: 'implementation',
            idsSkill: [],
            model: 'openai/b',
            maxTurns: null,
            skip: false,
            dispatched: false
        });
    });

    it('a turn limit is saved for that stage', async () => {
        const { editor } = await setup();

        editor().maxTurnsChanged.emit({ stage: AgentStage.Implementation, maxTurns: 90 });

        expect(agentRunApi.updateStagePlanEntry$).toHaveBeenCalledWith(
            55,
            expect.objectContaining({ name: 'implementation', maxTurns: 90 })
        );
    });

    it('a stage that already started is never saved', async () => {
        const { editor } = await setup();

        editor().modelPicked.emit({ stage: AgentStage.Design, model: 'openai/b' });

        expect(agentRunApi.updateStagePlanEntry$).not.toHaveBeenCalled();
    });

    it('switching the workflow reloads the stage plan so its skipped stages stay current', async () => {
        const { fixture } = await setup();
        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(1);

        fixture.nativeElement.querySelector('.workflow').click();

        expect(agentRunApi.updateWorkflow$).toHaveBeenCalled();
        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(2);
    });

    it('a 409 reloads the stage plan and warns that the stage just started', async () => {
        agentRunApi.updateStagePlanEntry$ = vi
            .fn()
            .mockReturnValue(throwError(() => ({ status: 409 })));
        const { editor } = await setup();
        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(1);

        editor().skillToggled.emit({ stage: AgentStage.Implementation, idSkill: 2 });

        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(2);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_STAGE_PLAN.RACE_ERROR');
    });

    it('any other failure reports a plain save error', async () => {
        agentRunApi.updateStagePlanEntry$ = vi
            .fn()
            .mockReturnValue(throwError(() => ({ status: 500 })));
        const { editor } = await setup();

        editor().skillToggled.emit({ stage: AgentStage.Implementation, idSkill: 2 });

        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_STAGE_PLAN.SAVE_ERROR');
    });
});
