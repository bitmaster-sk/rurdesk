import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { Subject, of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Fixtures } from 'src/testing/fixtures';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentRun, AgentStageProgress } from '../../model/agent-run.model';
import { AgentRunCardComponent } from './agent-run-card.component';

const FULL_PLAN = {
    stages: ['brainstorming', 'design', 'implementation_plan', 'implementation'].map(name => ({
        name,
        skippable: true,
        skip: false
    }))
};

function stages(design: AgentStageProgress['status']): AgentStageProgress[] {
    return [
        Fixtures.stageProgress({ stage: 'brainstorming', status: 'active', attempts: 1 }),
        Fixtures.stageProgress({ stage: 'design', status: design }),
        Fixtures.stageProgress({ stage: 'implementation_plan' }),
        Fixtures.stageProgress({ stage: 'implementation' })
    ];
}

describe('AgentRunCardComponent — short workflow (browser)', () => {
    let agentRunApi: { updateWorkflow$: ReturnType<typeof vi.fn> };
    let toast: { showError: ReturnType<typeof vi.fn> };

    beforeEach(() => {
        agentRunApi = { updateWorkflow$: vi.fn().mockReturnValue(of(Fixtures.agentRun())) };
        toast = { showError: vi.fn() };
    });

    async function setup(run: AgentRun) {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [AgentRunCardComponent],
            providers: [
                { provide: AgentRunApi, useValue: agentRunApi },
                { provide: SkillApi, useValue: { load$: vi.fn() } },
                { provide: ToastNotificationService, useValue: toast },
                { provide: NoticeService, useValue: { agentStats$: new Subject() } }
            ]
        })
            .overrideComponent(AgentRunCardComponent, {
                set: {
                    template: `
                        <input
                            type="checkbox"
                            [checked]="isShortWorkflow()"
                            [disabled]="isWorkflowLocked() || isSavingWorkflow()"
                            (click)="onToggleShortWorkflow($event)"
                        />
                    `
                }
            })
            .compileComponents();

        const fixture = TestBed.createComponent(AgentRunCardComponent);
        fixture.componentRef.setInput('run', run);
        fixture.detectChanges();
        return fixture;
    }

    function toggle(fixture: { nativeElement: HTMLElement }): HTMLInputElement {
        return fixture.nativeElement.querySelector('input') as HTMLInputElement;
    }

    it('turning it on patches the run before the design starts', async () => {
        const fixture = await setup(
            Fixtures.agentRun({ stagePlan: FULL_PLAN, stages: stages('pending') })
        );

        expect(toggle(fixture).checked).toBe(false);
        toggle(fixture).click();
        fixture.detectChanges();

        expect(agentRunApi.updateWorkflow$).toHaveBeenCalledWith(55, true);
        expect(toggle(fixture).checked).toBe(true);
    });

    it('shows the saved state of the run once it arrives', async () => {
        const fixture = await setup(
            Fixtures.agentRun({ stagePlan: FULL_PLAN, stages: stages('pending') })
        );

        fixture.componentRef.setInput(
            'run',
            Fixtures.agentRun({
                stagePlan: {
                    stages: FULL_PLAN.stages.map(entry => ({
                        ...entry,
                        skip: entry.name === 'design' || entry.name === 'implementation_plan'
                    }))
                },
                stages: stages('skipped')
            })
        );
        fixture.detectChanges();

        expect(toggle(fixture).checked).toBe(true);
    });

    it('is locked once the design has started', async () => {
        const fixture = await setup(
            Fixtures.agentRun({ stagePlan: FULL_PLAN, stages: stages('active') })
        );

        expect(toggle(fixture).disabled).toBe(true);
    });

    it('a 409 reverts the switch and warns that the next stage just started', async () => {
        agentRunApi.updateWorkflow$ = vi.fn().mockReturnValue(throwError(() => ({ status: 409 })));
        const fixture = await setup(
            Fixtures.agentRun({ stagePlan: FULL_PLAN, stages: stages('pending') })
        );

        toggle(fixture).click();
        fixture.detectChanges();

        expect(toggle(fixture).checked).toBe(false);
        expect(toast.showError).toHaveBeenCalledWith('AGENT.WORKFLOW.RACE_ERROR');
    });
});
