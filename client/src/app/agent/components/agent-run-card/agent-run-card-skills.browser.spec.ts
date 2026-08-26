import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import { AgentRunStageSkills } from '../../model/agent-run-skills.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Fixtures } from 'src/testing/fixtures';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { AgentRun } from '../../model/agent-run.model';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentRunCardComponent } from './agent-run-card.component';

const CATALOG: Skill[] = [
    Fixtures.skill({ idSkill: 1, name: 'Verification rules' }),
    Fixtures.skill({ idSkill: 2, name: 'TDD' })
];

const PAYLOAD: AgentRunStageSkills[] = [
    { name: AgentStage.Design, idsSkill: [1], dispatched: true },
    { name: AgentStage.Implementation, idsSkill: [], dispatched: false }
];

const RUN: AgentRun = Fixtures.agentRun();

abstract class Dom {
    public static chip(fixture: { nativeElement: HTMLElement }, key: string): HTMLButtonElement {
        return fixture.nativeElement.querySelector(`[data-chip="${key}"]`) as HTMLButtonElement;
    }
}

describe('AgentRunCardComponent — run skills (browser)', () => {
    let agentRunApi: {
        getAgentRunSkills$: ReturnType<typeof vi.fn>;
        patchAgentRunSkills$: ReturnType<typeof vi.fn>;
    };
    let skillApi: { load$: ReturnType<typeof vi.fn> };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        agentRunApi = {
            getAgentRunSkills$: vi.fn().mockReturnValue(of(PAYLOAD)),
            patchAgentRunSkills$: vi.fn().mockReturnValue(
                of([
                    { name: AgentStage.Design, idsSkill: [1], dispatched: true },
                    { name: AgentStage.Implementation, idsSkill: [2], dispatched: false }
                ])
            )
        };
        skillApi = { load$: vi.fn().mockReturnValue(of(CATALOG)) };
        toast = { showError: vi.fn() };
    });

    async function setup() {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [AgentRunCardComponent],
            providers: [
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: SkillApi, useValue: skillApi },
                { provide: ToastNotificationService, useValue: toast }
            ]
        })
            .overrideComponent(AgentRunCardComponent, {
                set: {
                    template: `
                        <button class="open" (click)="onOpenSkills()"></button>
                        @for (stage of runSkills(); track stage.name) {
                            <span class="status" [attr.data-stage]="stage.name">
                                {{ stageSaveStatus(stage.name) }}
                            </span>
                            @for (skill of skillCatalog(); track skill.idSkill) {
                                <button
                                    class="chip"
                                    [class.on]="isSkillOn(stage, skill.idSkill)"
                                    [disabled]="stage.dispatched"
                                    [attr.data-chip]="skill.idSkill + ':' + stage.name"
                                    (click)="onToggleSkill(stage, skill.idSkill)"
                                ></button>
                            }
                        }
                    `
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AgentRunCardComponent);
        fixture.componentRef.setInput('run', RUN);
        fixture.detectChanges();
        fixture.nativeElement.querySelector('.open').click();
        fixture.detectChanges();
        return fixture;
    }

    it('locks the chips of a stage that already started', async () => {
        const fixture = await setup();

        expect(Dom.chip(fixture, '1:design').disabled).toBe(true);
        expect(Dom.chip(fixture, '1:implementation').disabled).toBe(false);
        expect(Dom.chip(fixture, '1:design').classList).toContain('on');
    });

    it('toggling an editable chip patches that stage only', async () => {
        const fixture = await setup();

        Dom.chip(fixture, '2:implementation').click();
        fixture.detectChanges();

        expect(agentRunApi.patchAgentRunSkills$).toHaveBeenCalledWith(55, 'implementation', [2]);
        expect(Dom.chip(fixture, '2:implementation').classList).toContain('on');
        expect(
            fixture.nativeElement.querySelector('[data-stage="implementation"]').textContent?.trim()
        ).toBe(UiSaveState.Saved);
    });

    it('a 409 reloads the payload and warns that the stage just started', async () => {
        agentRunApi.patchAgentRunSkills$ = vi
            .fn()
            .mockReturnValue(throwError(() => ({ status: 409 })));
        const fixture = await setup();
        expect(agentRunApi.getAgentRunSkills$).toHaveBeenCalledTimes(1);

        Dom.chip(fixture, '2:implementation').click();
        fixture.detectChanges();

        expect(agentRunApi.getAgentRunSkills$).toHaveBeenCalledTimes(2);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_SKILLS.RACE_ERROR');
    });

    it('any other failure reports a plain save error', async () => {
        agentRunApi.patchAgentRunSkills$ = vi
            .fn()
            .mockReturnValue(throwError(() => ({ status: 500 })));
        const fixture = await setup();

        Dom.chip(fixture, '2:implementation').click();
        fixture.detectChanges();

        expect(toast.showError).toHaveBeenCalledWith('AGENT.RUN_SKILLS.SAVE_ERROR');
    });
});
