import { TestBed } from '@angular/core/testing';
import { Component, signal } from '@angular/core';
import { of } from 'rxjs';
import { vi, describe, it, expect } from 'vitest';
import { TranslateModule, TranslateLoader } from '@ngx-translate/core';
import { Observable, of as rxOf } from 'rxjs';
import {
    IssueParticipantModel,
    ParticipantSource
} from 'src/app/issue/model/issue-participant.model';
import { IssueParticipantService } from 'src/app/issue/service/issue-participant.service';
import { AvatarStub, TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
import { IssueParticipantsComponent } from './issue-participants.component';

// Stub ui-popover — the real one needs CDK overlay infrastructure not present here.
@Component({ selector: 'ui-popover', template: '<ng-content></ng-content>', standalone: false })
class PopoverStub {}

// Minimal translate loader that returns an empty catalogue — the translate
// pipe will fall back to the key, which is enough for DOM assertions.
class EmptyLoader implements TranslateLoader {
    public getTranslation(_lang: string): Observable<Record<string, string>> {
        return rxOf({});
    }
}

const makeParticipant = (
    overrides: Partial<IssueParticipantModel> = {}
): IssueParticipantModel => ({
    idUser: 1,
    name: 'Me',
    colorAvatarBg: '#000',
    isBot: false,
    source: ParticipantSource.Creator,
    hasNotificationsEnabled: true,
    ...overrides
});

function buildServiceStub(participants: IssueParticipantModel[] = []) {
    const _participants = signal(participants);
    return {
        participants: _participants.asReadonly(),
        load: vi.fn(),
        add$: vi.fn().mockReturnValue(of(undefined)),
        setMyNotifications$: vi.fn().mockReturnValue(of(undefined))
    };
}

async function setup(opts: { participants?: IssueParticipantModel[]; currentUserId?: number }) {
    const participants = opts.participants ?? [
        makeParticipant({ idUser: 1, name: 'Me', source: ParticipantSource.Creator }),
        makeParticipant({ idUser: 2, name: 'Other', source: ParticipantSource.Manual })
    ];
    const serviceStub = buildServiceStub(participants);

    await TestBed.configureTestingModule({
        declarations: [IssueParticipantsComponent, PopoverStub],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            AvatarStub,
            TablerIconStub,
            UiButtonStub,
            UiTooltipStub
        ],
        providers: [{ provide: IssueParticipantService, useValue: serviceStub }]
    })
        .overrideComponent(IssueParticipantsComponent, {
            // Remove the component-level IssueParticipantService provider so the
            // TestBed-level stub is injected instead of a fresh instance.
            set: { providers: [] }
        })
        .compileComponents();

    const fixture = TestBed.createComponent(IssueParticipantsComponent);
    const component = fixture.componentInstance;
    fixture.componentRef.setInput('idProject', 1);
    fixture.componentRef.setInput('idIssuePublic', 42);
    fixture.componentRef.setInput('idIssue', 100);
    fixture.componentRef.setInput('currentUserId', opts.currentUserId ?? 1);
    fixture.componentRef.setInput('members', []);

    // Expand the collapsible panel so participant rows are rendered.
    component.isCollapsed.set(false);
    fixture.detectChanges();

    return { fixture, component, serviceStub };
}

describe('IssueParticipantsComponent — notification toggle', () => {
    it('renders the notify toggle only for the current user row', async () => {
        const { fixture } = await setup({ currentUserId: 1 });
        const rows = fixture.nativeElement.querySelectorAll('.participant-row');
        expect(rows.length).toBe(2);

        // Row 0 is idUser=1 (current user) → toggle must be present
        const myToggle = rows[0].querySelector('.participant-row__notify-btn');
        expect(myToggle).not.toBeNull();

        // Row 1 is idUser=2 (other user) → toggle must be absent
        const otherToggle = rows[1].querySelector('.participant-row__notify-btn');
        expect(otherToggle).toBeNull();
    });

    it('clicking the toggle calls setMyNotifications$ with the negated value', async () => {
        const { fixture, serviceStub } = await setup({ currentUserId: 1 });
        const rows = fixture.nativeElement.querySelectorAll('.participant-row');
        const toggleBtn = rows[0].querySelector<HTMLButtonElement>('.participant-row__notify-btn');
        expect(toggleBtn).not.toBeNull();

        toggleBtn!.click();
        fixture.detectChanges();

        // Participant 1 has hasNotificationsEnabled=true → call should pass false
        expect(serviceStub.setMyNotifications$).toHaveBeenCalledWith(1, 42, false);
    });

    it('does NOT render any toggle when currentUserId does not match any participant', async () => {
        const { fixture } = await setup({ currentUserId: 99 }); // 99 is not among participants
        const toggleBtns = fixture.nativeElement.querySelectorAll('.participant-row__notify-btn');
        expect(toggleBtns.length).toBe(0);
    });
});

describe('IssueParticipantsComponent — role badges', () => {
    it('assigns creator badge class for ParticipantSource.Creator', async () => {
        const { fixture } = await setup({
            participants: [makeParticipant({ idUser: 1, source: ParticipantSource.Creator })],
            currentUserId: 99
        });
        const badge = fixture.nativeElement.querySelector('.role-badge');
        expect(badge?.classList.contains('role-badge--creator')).toBe(true);
    });

    it('assigns bot badge class when isBot is true regardless of source', async () => {
        const { fixture } = await setup({
            participants: [
                makeParticipant({ idUser: 1, isBot: true, source: ParticipantSource.Creator })
            ],
            currentUserId: 99
        });
        const badge = fixture.nativeElement.querySelector('.role-badge');
        expect(badge?.classList.contains('role-badge--bot')).toBe(true);
    });
});
