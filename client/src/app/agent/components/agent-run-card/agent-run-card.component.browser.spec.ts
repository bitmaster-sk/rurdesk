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
                RunWikiContextStub
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
});
