import { ComponentFixture, TestBed } from '@angular/core/testing';
import { RouterModule, provideRouter } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { AvatarStub, TablerIconStub, UiButtonStub } from 'src/testing/stubs';
import { NotificationType } from '../../model/notification-type.enum';
import { Notification } from '../../model/notification.model';
import { NotificationCardComponent } from './notification-card.component';

function notification(overrides: Partial<Notification> = {}): Notification {
    return {
        idNotification: 1,
        type: NotificationType.WikiProposalReady,
        idProject: 7,
        projectName: 'Rurdesk',
        actorName: 'Kimi',
        refType: 'issue',
        refId: '42',
        refTitle: 'Reuse the branch',
        refPublicId: 3,
        body: { count: 3, idProposal: 15, title: 'Architecture' },
        isRead: false,
        createdAt: '2026-10-08T10:00:00Z',
        ...overrides
    };
}

describe('NotificationCardComponent', () => {
    let fixture: ComponentFixture<NotificationCardComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [NotificationCardComponent],
            imports: [
                TranslateModule.forRoot(),
                RouterModule,
                AvatarStub,
                TablerIconStub,
                UiButtonStub
            ],
            providers: [provideRouter([])]
        }).compileComponents();
        fixture = TestBed.createComponent(NotificationCardComponent);
    });

    function render(value: Notification): HTMLElement {
        fixture.componentRef.setInput('notification', value);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function link(el: HTMLElement, testId: string): HTMLAnchorElement | null {
        return el.querySelector<HTMLAnchorElement>(`[data-testid="${testId}"]`);
    }

    it('links both the task and the first ready wiki proposal', () => {
        const el = render(notification());

        expect(el.querySelector('[data-testid="notif-wiki-proposal"] strong')?.textContent).toBe(
            'Kimi'
        );
        expect(link(el, 'notif-wiki-proposal-task')?.getAttribute('href')).toBe(
            '/project/7/issue/3'
        );
        expect(link(el, 'notif-wiki-proposal-task')?.textContent).toContain('#3 Reuse the branch');
        expect(link(el, 'notif-wiki-proposal-link')?.getAttribute('href')).toBe(
            '/project/7/wiki/proposals/15'
        );
        expect(link(el, 'notif-wiki-proposal-link')?.textContent).toContain('Architecture');
        expect(el.textContent).toContain('NOTIFICATION.WIKI_PROPOSAL.MORE');
    });

    it('sends an older notification without a proposal id to the proposal list', () => {
        const el = render(notification({ body: { count: 1 } }));

        expect(link(el, 'notif-wiki-proposal-link')?.getAttribute('href')).toBe(
            '/project/7/wiki/proposals'
        );
        expect(el.textContent).not.toContain('NOTIFICATION.WIKI_PROPOSAL.MORE');
    });

    it('links the proposal that needs resolving after the merge', () => {
        const el = render(
            notification({
                type: NotificationType.WikiProposalConflict,
                body: { count: 1, idProposal: 15, title: 'Architecture' }
            })
        );

        expect(el.textContent).toContain('NOTIFICATION.TEXT.WIKI_PROPOSAL_CONFLICT');
        expect(link(el, 'notif-wiki-proposal-link')?.getAttribute('href')).toBe(
            '/project/7/wiki/proposals/15'
        );
        expect(el.textContent).toContain('NOTIFICATION.TYPE.WIKI_PROPOSAL_CONFLICT');
    });

    it('keeps the one-line text for other notifications', () => {
        const el = render(notification({ type: NotificationType.Assigned, body: undefined }));

        expect(el.querySelector('[data-testid="notif-wiki-proposal"]')).toBeNull();
        expect(el.textContent).toContain('NOTIFICATION.TEXT.ASSIGNED');
    });

    it('says who added the user to which team', () => {
        const el = render(
            notification({
                type: NotificationType.TeamJoined,
                idProject: undefined,
                projectName: undefined,
                refType: 'team',
                refId: '5',
                refTitle: 'Backend',
                refPublicId: undefined,
                body: undefined
            })
        );

        expect(el.querySelector('strong')?.textContent).toBe('Kimi');
        expect(el.textContent).toContain('NOTIFICATION.TEXT.TEAM_JOINED');
        expect(el.querySelector('[data-testid="notif-team-name"]')?.textContent).toBe('Backend');
    });
});
