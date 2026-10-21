import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Fixtures } from 'src/testing/fixtures';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentStageCardView } from '../../entity/agent-stage-card.entity';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';
import { AgentPhase } from '../../model/agent-phase.enum';
import { AgentRunStagePlanRes } from '../../model/agent-run-stage-plan.model';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentRunCardComponent } from './agent-run-card.component';

@Component({ selector: 'app-agent-stage-card', template: '', standalone: true })
class AgentStageCardStub {
    public readonly view = input.required<AgentStageCardView>();
    public readonly planRow = input<AgentStagePlanEditorRow | null>(null);
    public readonly saveState = input<UiSaveState>(UiSaveState.Idle);
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

const RUN = Fixtures.agentRun({
    phase: AgentPhase.InProgress,
    stages: [
        Fixtures.stageProgress({ stage: AgentStage.Design, status: 'done', attempts: 1 }),
        Fixtures.stageProgress({ stage: AgentStage.ImplementationPlan, status: 'skipped' }),
        Fixtures.stageProgress({ stage: AgentStage.Implementation })
    ]
});

describe('AgentRunCardComponent — run stage plan (browser)', () => {
    let agentRunApi: {
        loadStagePlan$: ReturnType<typeof vi.fn>;
        updateStagePlanEntry$: ReturnType<typeof vi.fn>;
        updateWorkflow$: ReturnType<typeof vi.fn>;
        loadStats$: ReturnType<typeof vi.fn>;
        loadWikiReads$: ReturnType<typeof vi.fn>;
    };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        agentRunApi = {
            loadStagePlan$: vi.fn().mockReturnValue(of(stagePlan([]))),
            updateStagePlanEntry$: vi.fn().mockReturnValue(of(stagePlan([2]))),
            updateWorkflow$: vi.fn().mockReturnValue(of(Fixtures.agentRun())),
            loadStats$: vi.fn().mockReturnValue(of(null)),
            loadWikiReads$: vi.fn().mockReturnValue(of([]))
        };
        toast = { showError: vi.fn() };
    });

    async function setup(
        run = RUN,
        isOpenedByUser = true
    ): Promise<{
        fixture: ComponentFixture<AgentRunCardComponent>;
        card: (stage: string) => AgentStageCardStub;
    }> {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), AgentStageCardStub],
            declarations: [AgentRunCardComponent],
            providers: [
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: SkillApi, useValue: { load$: vi.fn().mockReturnValue(of(CATALOG)) } },
                { provide: ToastNotificationService, useValue: toast },
                { provide: NoticeService, useValue: { agentStats$: new Subject() } }
            ]
        })
            .overrideComponent(AgentRunCardComponent, {
                set: {
                    template: `
                        <button class="open" (click)="onToggle()"></button>
                        <button class="workflow" (click)="onToggleShortWorkflow($event)"></button>
                        @if (isOpen()) {
                            @for (card of stageCards(); track card.stage) {
                                <app-agent-stage-card
                                    [view]="card"
                                    [planRow]="stagePlanRows().get(card.stage) ?? null"
                                    [saveState]="stageSaveState(card.stage)"
                                    (skillToggled)="onToggleSkill($event)"
                                    (modelPicked)="onPickStageModel($event)"
                                    (maxTurnsChanged)="onStageMaxTurns($event)"
                                />
                            }
                        }
                    `
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AgentRunCardComponent);
        fixture.componentRef.setInput('run', run);
        fixture.detectChanges();
        if (isOpenedByUser) {
            fixture.nativeElement.querySelector('.open').click();
            fixture.detectChanges();
        }
        const card = (stage: string) =>
            fixture.debugElement
                .queryAll(By.directive(AgentStageCardStub))
                .map(found => found.componentInstance as AgentStageCardStub)
                .find(found => found.view().stage === stage)!;
        return { fixture, card };
    }

    it('loads the plan once the card opens and offers the default turns per stage', async () => {
        const { card } = await setup();

        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledWith(55);
        expect(card(AgentStage.Design).planRow()?.defaultMaxTurns).toBe(250);
        expect(card(AgentStage.ImplementationPlan).planRow()?.lock).toBe(
            AgentStagePlanEditorLock.Skipped
        );
        expect(card(AgentStage.Implementation).planRow()).toEqual({
            stage: AgentStage.Implementation,
            idsSkill: [],
            model: null,
            maxTurns: null,
            defaultMaxTurns: 500,
            lock: null
        });
    });

    it('loads the plan once a failed run is continued, so its next stage can be edited', async () => {
        const { fixture, card } = await setup({ ...RUN, phase: AgentPhase.Failed }, false);
        expect(agentRunApi.loadStagePlan$).not.toHaveBeenCalled();

        fixture.componentRef.setInput('run', { ...RUN, phase: AgentPhase.Queued });
        fixture.detectChanges();
        fixture.nativeElement.querySelector('.open').click();
        fixture.detectChanges();

        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledWith(55);
        expect(card(AgentStage.Implementation).planRow()?.stage).toBe(AgentStage.Implementation);
    });

    it('toggling a skill saves that stage and marks it saved', async () => {
        const { fixture, card } = await setup();

        card(AgentStage.Implementation).skillToggled.emit({
            stage: AgentStage.Implementation,
            idSkill: 2
        });
        fixture.detectChanges();

        expect(agentRunApi.updateStagePlanEntry$).toHaveBeenCalledWith(
            55,
            expect.objectContaining({ name: 'implementation', idsSkill: [2] })
        );
        expect(card(AgentStage.Implementation).planRow()?.idsSkill).toEqual([2]);
        expect(card(AgentStage.Implementation).saveState()).toBe(UiSaveState.Saved);
    });

    it('picking a model keeps the stage skills and saves the model', async () => {
        const { card } = await setup();

        card(AgentStage.Implementation).modelPicked.emit({
            stage: AgentStage.Implementation,
            model: 'openai/b'
        });

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
        const { card } = await setup();

        card(AgentStage.Implementation).maxTurnsChanged.emit({
            stage: AgentStage.Implementation,
            maxTurns: 90
        });

        expect(agentRunApi.updateStagePlanEntry$).toHaveBeenCalledWith(
            55,
            expect.objectContaining({ name: 'implementation', maxTurns: 90 })
        );
    });

    it('a stage that already started is never saved', async () => {
        const { card } = await setup();

        card(AgentStage.Design).modelPicked.emit({ stage: AgentStage.Design, model: 'openai/b' });

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
        const { card } = await setup();
        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(1);

        card(AgentStage.Implementation).skillToggled.emit({
            stage: AgentStage.Implementation,
            idSkill: 2
        });

        expect(agentRunApi.loadStagePlan$).toHaveBeenCalledTimes(2);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_STAGE_PLAN.RACE_ERROR');
    });

    it('any other failure reports a plain save error', async () => {
        agentRunApi.updateStagePlanEntry$ = vi
            .fn()
            .mockReturnValue(throwError(() => ({ status: 500 })));
        const { card } = await setup();

        card(AgentStage.Implementation).skillToggled.emit({
            stage: AgentStage.Implementation,
            idSkill: 2
        });

        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_STAGE_PLAN.SAVE_ERROR');
    });
});
