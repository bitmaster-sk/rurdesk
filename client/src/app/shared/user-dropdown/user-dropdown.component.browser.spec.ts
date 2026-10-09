import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { AgentRunApi } from '../../agent/api/agent-run.api.service';
import { AgentOverview } from '../../agent/model/agent-overview.model';
import { UserDropdownComponent } from './user-dropdown.component';

const ZOE: User = { idUser: 2, name: 'Zoe', email: 'z@z.sk', colorAvatarBg: '#222' };

const OVERVIEW: AgentOverview[] = [
    {
        idUserAgent: 2,
        isBusy: false,
        current: null,
        queueCount: 0,
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
    public readonly users = signal<User[]>([ZOE]);
    public readonly ctrl = new FormControl<number | null>(null);
}

describe('UserDropdownComponent OnPush re-render (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            declarations: [HostComponent, UserDropdownComponent],
            providers: [
                {
                    provide: AgentRunApi,
                    useValue: { loadAgentsOverview$: () => of(OVERVIEW) }
                }
            ]
        })
            .overrideComponent(UserDropdownComponent, {
                set: {
                    template: `
                        @for (user of sortedUsers(); track user.idUser) {
                            <span class="name">{{ user.name }}</span>
                        }
                        <button class="open" (click)="onOpened()"></button>
                        <span class="busy">{{ overviewOf(2)?.isBusy }}</span>
                    `
                }
            })
            .compileComponents();
    });

    it('re-renders after async agent overview is loaded', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        fixture.nativeElement.querySelector('.open')!.click();
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('.busy').textContent).toBe('false');
    });
});
