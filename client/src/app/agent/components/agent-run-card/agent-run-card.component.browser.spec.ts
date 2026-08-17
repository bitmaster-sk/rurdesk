import { Component, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';

import { AgentRunCardComponent } from './agent-run-card.component';
import { AgentPhase } from '../../model/agent-phase.enum';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';

@Component({ selector: 'app-run-recovery-banner', template: '', standalone: true })
class RunRecoveryBannerStub {
    public readonly idRun = input<number>(0);
    public readonly errorKey = input<string | null>(null);
    public readonly errorDetail = input<string | null>(null);
    public readonly continued = output<void>();
    public readonly restarted = output<void>();
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
                RunStatsPanelStub
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
        } as Partial<AgentRun> as AgentRun);
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
});
