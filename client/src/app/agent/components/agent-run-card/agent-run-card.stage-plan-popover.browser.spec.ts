import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Subject, of } from 'rxjs';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { AgentRunCardComponent } from './agent-run-card.component';
import { AgentPhase } from '../../model/agent-phase.enum';
import { AgentStage } from '../../model/agent-stage.enum';
import { AgentRunStagePlanRes } from '../../model/agent-run-stage-plan.model';
import { AgentStagePlanEditorComponent } from '../agent-stage-plan-editor/agent-stage-plan-editor.component';
import { FormsModule } from '@angular/forms';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { Fixtures } from 'src/testing/fixtures';
import { Skill } from 'src/app/shared/model/skill.model';

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

@Component({ selector: 'app-run-stats-panel', template: '', standalone: true })
class RunStatsPanelStub {
    public readonly idRun = input<number>(0);
}

const RUN_STAGE_PLAN: AgentRunStagePlanRes = {
    stages: [
        {
            name: AgentStage.Brainstorming,
            idsSkill: [1],
            model: null,
            maxTurns: null,
            skip: false,
            dispatched: false
        },
        {
            name: AgentStage.Design,
            idsSkill: [2, 3],
            model: null,
            maxTurns: null,
            skip: false,
            dispatched: false
        },
        {
            name: AgentStage.ImplementationPlan,
            idsSkill: [1, 2, 3],
            model: null,
            maxTurns: null,
            skip: false,
            dispatched: false
        },
        {
            name: AgentStage.Implementation,
            idsSkill: [1],
            model: null,
            maxTurns: null,
            skip: false,
            dispatched: false
        }
    ],
    models: [],
    modelDefault: null,
    maxTurnsPlan: 250,
    maxTurnsImplement: 500,
    maxTurnsLimit: null
};

describe('AgentRunCardComponent — stage plan popover positioning (browser)', () => {
    let fixture: ComponentFixture<AgentRunCardComponent>;
    let stagePlanSubject: Subject<AgentRunStagePlanRes>;
    let catalogSubject: Subject<Skill[]>;

    beforeEach(async () => {
        stagePlanSubject = new Subject();
        catalogSubject = new Subject();

        const agentRunApiStub = {
            loadStagePlan$: vi.fn(() => stagePlanSubject.asObservable()),
            updateStagePlanEntry$: vi.fn(() => of(RUN_STAGE_PLAN))
        };

        const skillApiStub = {
            load$: vi.fn(() => catalogSubject.asObservable())
        };

        const toastStub = { showError: vi.fn() };

        await TestBed.configureTestingModule({
            imports: [
                TranslateModule.forRoot(),
                FormsModule,
                UiModule,
                TablerIconStub,
                RunRecoveryBannerStub,
                RunStatsPanelStub,
                RunWikiContextStub
            ],
            declarations: [AgentRunCardComponent, AgentStagePlanEditorComponent],
            providers: [
                { provide: AgentRunApi, useValue: agentRunApiStub },
                { provide: SkillApi, useValue: skillApiStub },
                { provide: ToastNotificationService, useValue: toastStub },
                provideNoopAnimations()
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(AgentRunCardComponent);
        fixture.componentRef.setInput(
            'run',
            Fixtures.agentRun({ phase: AgentPhase.AwaitingApproval })
        );
        fixture.detectChanges();
    });

    function openStagePlanPopover(): HTMLElement {
        const chip = fixture.nativeElement.querySelector(
            '[data-testid="run-stage-plan-edit"]'
        ) as HTMLElement;
        chip.click();
        fixture.detectChanges();
        return chip;
    }

    function getPane(): HTMLElement {
        return document.querySelector('.cdk-overlay-pane') as HTMLElement;
    }

    it('repositions above the trigger after async content loads when trigger is near viewport bottom', async () => {
        const chip = fixture.nativeElement.querySelector(
            '[data-testid="run-stage-plan-edit"]'
        ) as HTMLElement;
        expect(chip).not.toBeNull();

        Object.defineProperty(chip, 'getBoundingClientRect', {
            value: () => ({
                top: window.innerHeight - 40,
                bottom: window.innerHeight - 20,
                left: 100,
                right: 200,
                width: 100,
                height: 20
            }),
            configurable: true
        });

        chip.click();
        fixture.detectChanges();

        const pane = getPane();
        expect(pane).not.toBeNull();

        const initialTop = pane.getBoundingClientRect().top;

        catalogSubject.next([
            Fixtures.skill(),
            Fixtures.skill({ idSkill: 2, name: 'Code review' }),
            Fixtures.skill({ idSkill: 3, name: 'Testing' })
        ]);
        stagePlanSubject.next(RUN_STAGE_PLAN);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const repositionedTop = pane.getBoundingClientRect().top;
        expect(repositionedTop).toBeLessThan(initialTop);
    });

    it('tracks the trigger when the cdkScrollable container scrolls', async () => {
        const scrollContainer = document.createElement('div');
        scrollContainer.style.cssText = 'height: 200px; overflow-y: auto;';
        const tallContent = document.createElement('div');
        tallContent.style.cssText = 'height: 600px;';
        scrollContainer.appendChild(tallContent);

        const host = fixture.nativeElement as HTMLElement;
        const originalParent = host.parentElement;
        scrollContainer.appendChild(host);
        document.body.appendChild(scrollContainer);

        try {
            const chip = openStagePlanPopover();
            const pane = getPane();
            expect(pane).not.toBeNull();

            const initialPaneTop = pane.getBoundingClientRect().top;
            const initialChipTop = chip.getBoundingClientRect().top;

            scrollContainer.scrollTop = 50;
            scrollContainer.dispatchEvent(new Event('scroll', { bubbles: true }));
            fixture.detectChanges();
            await fixture.whenStable();
            fixture.detectChanges();

            const scrolledPaneTop = pane.getBoundingClientRect().top;
            const scrolledChipTop = chip.getBoundingClientRect().top;

            expect(scrolledChipTop).toBeLessThan(initialChipTop);
            expect(scrolledPaneTop).not.toBe(initialPaneTop);
        } finally {
            if (originalParent) {
                originalParent.appendChild(host);
            }
            scrollContainer.remove();
        }
    });
});
