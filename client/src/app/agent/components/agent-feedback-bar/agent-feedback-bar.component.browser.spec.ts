import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule } from '@ngx-translate/core';
import { beforeEach, describe, expect, it } from 'vitest';
import { Fixtures } from 'src/testing/fixtures';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from '../../../ui/ui.module';
import { UiTooltipDirective } from '../../../ui/directives/tooltip.directive';
import { AgentPhase } from '../../model/agent-phase.enum';
import { AgentRun } from '../../model/agent-run.model';
import { AgentFeedback } from '../../entity/agent-feedback.entity';
import { AgentFeedbackBarComponent } from './agent-feedback-bar.component';

const NOTHING_NEW: AgentFeedback = { comments: 0, replies: 0, threads: 0 };
const SIX_NEW: AgentFeedback = { comments: 2, replies: 4, threads: 2 };

function waitingPlan(overrides: Partial<AgentRun> = {}): AgentRun {
    return Fixtures.agentRun({
        phase: AgentPhase.AwaitingApproval,
        stages: [
            Fixtures.stageProgress({ stage: 'implementation_plan', status: 'awaiting_approval' })
        ],
        ...overrides
    });
}

describe('AgentFeedbackBarComponent (browser)', () => {
    let fixture: ComponentFixture<AgentFeedbackBarComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [AgentFeedbackBarComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconStub]
        }).compileComponents();
        fixture = TestBed.createComponent(AgentFeedbackBarComponent);
        fixture.componentRef.setInput('agentName', 'Kimi');
    });

    function render(run: AgentRun | null, feedback: AgentFeedback, hasMockups = false): void {
        fixture.componentRef.setInput('run', run);
        fixture.componentRef.setInput('feedback', feedback);
        fixture.componentRef.setInput('hasMockups', hasMockups);
        fixture.detectChanges();
    }

    function find(testId: string): HTMLElement | null {
        return fixture.nativeElement.querySelector(`[data-testid="${testId}"]`);
    }

    it('shows how much is new and Send while the plan waits with new comments', () => {
        render(waitingPlan(), SIX_NEW);

        expect(find('agent-feedback-bar-count')?.textContent).toContain('AGENT.FEEDBACK_BAR.NEW');
        expect(find('agent-feedback-send')).not.toBeNull();
        expect(find('agent-feedback-approve')).toBeNull();
    });

    it('approves anyway next to Send only after the confirmation', () => {
        let approved = 0;
        fixture.componentInstance.approve.subscribe(() => approved++);
        render(waitingPlan(), SIX_NEW);

        find('agent-feedback-approve-anyway')?.querySelector('button')?.click();
        fixture.detectChanges();
        expect(approved).toBe(0);
        const panel = document.querySelector('.ui-confirm-panel');
        expect(panel?.textContent).toContain('AGENT.FEEDBACK_BAR.APPROVE_ANYWAY_CONFIRM');

        // Template order: reject first, accept second.
        (panel?.querySelectorAll('button')[1] as HTMLButtonElement).click();
        expect(approved).toBe(1);
    });

    it('offers no Approve anyway for mockups, a question or a pull request', () => {
        render(waitingPlan(), SIX_NEW, true);
        expect(find('agent-feedback-send')).not.toBeNull();
        expect(find('agent-feedback-approve-anyway')).toBeNull();

        render(Fixtures.agentRun({ phase: AgentPhase.AwaitingInput }), SIX_NEW);
        expect(find('agent-feedback-approve-anyway')).toBeNull();

        render(Fixtures.agentRun({ phase: AgentPhase.PrOpen }), SIX_NEW);
        expect(find('agent-feedback-approve-anyway')).toBeNull();
    });

    it('offers Approve when nothing is new', () => {
        render(waitingPlan(), NOTHING_NEW);

        expect(find('agent-feedback-approve')).not.toBeNull();
        expect(find('agent-feedback-send')).toBeNull();
        expect(find('agent-feedback-bar-count')).toBeNull();
    });

    it('shows the mockup hint and no button for a design with mockups', () => {
        render(waitingPlan(), NOTHING_NEW, true);

        expect(find('agent-feedback-bar-text')?.textContent).toContain(
            'AGENT.FEEDBACK_BAR.CHOOSE_MOCKUP'
        );
        expect(fixture.nativeElement.querySelector('ui-button')).toBeNull();
    });

    it('shows a question without a button and nothing for an open PR with nothing new', () => {
        render(Fixtures.agentRun({ phase: AgentPhase.AwaitingInput }), NOTHING_NEW);
        expect(find('agent-feedback-bar-text')?.textContent).toContain(
            'AGENT.FEEDBACK_BAR.ASKED_QUESTION'
        );
        expect(fixture.nativeElement.querySelector('ui-button')).toBeNull();

        render(Fixtures.agentRun({ phase: AgentPhase.PrOpen }), NOTHING_NEW);
        expect(find('agent-feedback-bar')).toBeNull();
    });

    it('shows nothing while the agent works, after the run or while a person holds the task', () => {
        for (const phase of [
            AgentPhase.InProgress,
            AgentPhase.Queued,
            AgentPhase.Done,
            AgentPhase.Failed
        ]) {
            render(Fixtures.agentRun({ phase }), SIX_NEW);
            expect(find('agent-feedback-bar')).toBeNull();
        }
        render(waitingPlan({ isParked: true }), SIX_NEW);
        expect(find('agent-feedback-bar')).toBeNull();
    });

    it('sends once and is disabled while the send runs', () => {
        let sent = 0;
        fixture.componentInstance.send.subscribe(() => sent++);
        render(waitingPlan(), SIX_NEW);

        find('agent-feedback-send')?.querySelector('button')?.click();
        expect(sent).toBe(1);

        fixture.componentRef.setInput('isSending', true);
        fixture.detectChanges();
        const button = find('agent-feedback-send')?.querySelector('button');
        expect(button?.disabled).toBe(true);
        button?.click();
        expect(sent).toBe(1);
    });

    it('breaks the count down in the tooltip of Send', () => {
        render(waitingPlan(), SIX_NEW);

        const tooltip = fixture.debugElement
            .query(By.css('[data-testid="agent-feedback-send"]'))
            .injector.get(UiTooltipDirective);
        expect(tooltip.uiTooltip()).toBe('AGENT.FEEDBACK_BAR.BREAKDOWN');
    });
});
