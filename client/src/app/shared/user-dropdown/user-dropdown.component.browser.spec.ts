import { Component, signal } from '@angular/core';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { Fixtures } from 'src/testing/fixtures';
import { AgentRunApi } from '../../agent/api/agent-run.api.service';
import { AgentRun } from '../../agent/model/agent-run.model';
import { AgentOverview } from '../../agent/model/agent-overview.model';
import { UserDropdownComponent } from './user-dropdown.component';

const ADA: User = Fixtures.user({ idUser: 1, name: 'Ada' });
const ZOE: User = Fixtures.user({ idUser: 2, name: 'Zoe' });
const AGENT: User = Fixtures.agent();
const RUN: AgentRun = Fixtures.agentRun();

const OVERVIEW: AgentOverview[] = [
    {
        idUserAgent: AGENT.idUser,
        isBusy: true,
        current: null,
        queueCount: 2,
        queuedIdsIssuePublic: [],
        completedToday: 0,
        tokens7d: 0,
        avgRunDurationMs7d: null,
        failedAttempts7d: 0
    }
];

@Component({
    standalone: false,
    template: `
        <app-user-dropdown
            [users]="users()"
            [formControl]="ctrl"
            [hasAgentFeatures]="true"
            [idProject]="7"
            [idIssuePublic]="42"
        />
    `
})
class HostComponent {
    public readonly users = signal<User[]>([ADA, ZOE, AGENT]);
    public readonly ctrl = new FormControl<number | null>(null);
}

describe('UserDropdownComponent OnPush re-render (browser)', () => {
    let agentRunApi: { loadAgentsOverview$: ReturnType<typeof vi.fn> };

    beforeEach(async () => {
        agentRunApi = { loadAgentsOverview$: vi.fn().mockReturnValue(of(OVERVIEW)) };

        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            declarations: [HostComponent, UserDropdownComponent],
            providers: [{ provide: AgentRunApi, useValue: agentRunApi }]
        })
            .overrideComponent(UserDropdownComponent, {
                set: {
                    template: `
                        <span class="selected">{{ selected ?? 'none' }}</span>
                        <button class="open" (click)="onOpened()"></button>
                        <span class="busy">{{ overviewOf(${AGENT.idUser})?.isBusy }}</span>
                    `
                }
            })
            .compileComponents();
    });

    it('re-renders the selected value after writeValue via FormControl.setValue', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.selected')!.textContent).toBe('none');

        fixture.componentInstance.ctrl.setValue(ADA.idUser);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.selected')!.textContent).toBe(
            String(ADA.idUser)
        );
    });

    it('re-renders the selected value after onAgentRunCreated', () => {
        const fixture = TestBed.createComponent(HostComponent);
        const dropdown = fixture.debugElement.children[0]
            .componentInstance as UserDropdownComponent;

        fixture.componentInstance.ctrl.setValue(ZOE.idUser);
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.selected')!.textContent).toBe(
            String(ZOE.idUser)
        );

        (dropdown as unknown as { onAgentRunCreated: (run: AgentRun) => void }).onAgentRunCreated(
            RUN
        );
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.selected')!.textContent).toBe(
            String(AGENT.idUser)
        );
    });

    it('re-renders after async agent overview is loaded', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        fixture.nativeElement.querySelector('.open')!.click();
        fixture.detectChanges();

        expect(agentRunApi.loadAgentsOverview$).toHaveBeenCalledWith(7);
        expect(fixture.nativeElement.querySelector('.busy')!.textContent).toBe('true');
    });
});
