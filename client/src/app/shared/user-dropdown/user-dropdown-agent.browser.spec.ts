import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { Fixtures } from 'src/testing/fixtures';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRunApi } from '../../agent/api/agent-run.api.service';
import { AgentRun } from '../../agent/model/agent-run.model';
import { AgentOverview } from '../../agent/model/agent-overview.model';
import { UserDropdownComponent } from './user-dropdown.component';

const ADA: User = Fixtures.user({ idUser: 1, name: 'Ada' });
const ZOE: User = Fixtures.user({ idUser: 2, name: 'Zoe' });
const BOT: User = Fixtures.bot();
const RUN: AgentRun = Fixtures.agentRun();

const OVERVIEW: AgentOverview[] = [
    {
        idUserBot: 8,
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
            (agentRunCreated)="lastAssigned = $event"
        />
    `
})
class HostComponent {
    public readonly users = signal<User[]>([]);
    public readonly ctrl = new FormControl<number | null>(null);
    public lastAssigned: AgentRun | null = null;
}

abstract class Dom {
    public static names(fixture: { nativeElement: HTMLElement }): (string | null)[] {
        return Array.from(fixture.nativeElement.querySelectorAll('.name')).map(
            (el: Element) => el.textContent
        );
    }

    public static click(fixture: { nativeElement: HTMLElement }, selector: string): void {
        (fixture.nativeElement.querySelector(selector) as HTMLElement).click();
    }
}

describe('UserDropdownComponent bot features (browser)', () => {
    let agentRunApi: { agentsOverview$: ReturnType<typeof vi.fn> };

    beforeEach(async () => {
        agentRunApi = { agentsOverview$: vi.fn().mockReturnValue(of(OVERVIEW)) };

        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            declarations: [HostComponent, UserDropdownComponent],
            providers: [{ provide: AgentRunApi, useValue: agentRunApi }]
        })
            .overrideComponent(UserDropdownComponent, {
                set: {
                    template: `
                        @for (user of sortedUsers(); track user.idUser) {
                            <span class="name">{{ user.name }}</span>
                        }
                        <span class="busy">{{ overviewOf(8)?.isBusy }}</span>
                        <button class="open" (click)="onOpened()"></button>
                    `
                }
            })
            .compileComponents();
    });

    function setup(users: User[] = [BOT, ZOE, ADA]) {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.users.set(users);
        fixture.detectChanges();
        return fixture;
    }

    it('sorts bots after humans, by name inside each group', () => {
        const fixture = setup();

        expect(Dom.names(fixture)).toEqual(['Ada', 'Zoe', 'ci-bot']);
    });

    it('renders members that arrive after the first render', () => {
        // The member list comes from an async store, so the very first render
        // always sees an empty array — a cached view of it leaves the dropdown
        // permanently empty and nobody can be assigned.
        const fixture = setup([]);
        expect(Dom.names(fixture)).toEqual([]);

        fixture.componentInstance.users.set([BOT, ADA]);
        fixture.detectChanges();

        expect(Dom.names(fixture)).toEqual(['Ada', 'ci-bot']);
    });

    it('loads the workload only when the panel opens', () => {
        const fixture = setup();
        expect(agentRunApi.agentsOverview$).not.toHaveBeenCalled();

        Dom.click(fixture, '.open');
        fixture.detectChanges();

        expect(agentRunApi.agentsOverview$).toHaveBeenCalledWith(7);
        expect(fixture.nativeElement.querySelector('.busy').textContent).toBe('true');
    });

    it('a dock assignment updates the value without firing the form change', () => {
        const fixture = setup();
        const dropdown = fixture.debugElement.children[0]
            .componentInstance as UserDropdownComponent;
        let changeCount = 0;
        fixture.componentInstance.ctrl.valueChanges.subscribe(() => changeCount++);

        (dropdown as unknown as { onAgentRunCreated: (run: AgentRun) => void }).onAgentRunCreated(
            RUN
        );
        fixture.detectChanges();

        // The server already persisted the assignment; emitting here would
        // re-enter the assignee hook and create a second run.
        expect(changeCount).toBe(0);
        expect(fixture.componentInstance.lastAssigned).toEqual(RUN);
        expect(dropdown.value).toBe(8);
    });
});
